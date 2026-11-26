package mattermost

import (
	"archive/zip"
	"bytes"
	"encoding/base64"
	"encoding/json"
	"image"
	"image/color"
	"image/png"
	"io"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

func TestAgentGeneratesImageAndEmbedsItInPowerPoint(t *testing.T) {
	f, _, _ := agentsFixture(t)
	picture := image.NewRGBA(image.Rect(0, 0, 80, 48))
	picture.Set(1, 1, color.RGBA{R: 42, G: 110, B: 68, A: 255})
	var pngBytes bytes.Buffer
	if err := png.Encode(&pngBytes, picture); err != nil {
		t.Fatal(err)
	}
	var imageRequests atomic.Int32
	generator := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		imageRequests.Add(1)
		if r.URL.Path != "/v1/images/generations" || r.Header.Get("Authorization") != "Bearer fixture-image-key" {
			t.Error("wrong image provider request")
		}
		var request map[string]any
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Error(err)
		}
		if request["model"] != "fixture-image" || request["size"] != "1536x1024" ||
			request["output_format"] != "png" || request["prompt"] != "Книги на столе" {
			t.Errorf("wrong image request payload: %v", request)
		}
		write(w, 200, map[string]any{"data": []map[string]string{{"b64_json": base64.StdEncoding.EncodeToString(pngBytes.Bytes())}}})
	}))
	defer generator.Close()

	model := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var input struct {
			Messages []agentMessage
			Tools    []struct{ Function agentFunction }
		}
		if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
			t.Error(err)
			return
		}
		addTool := func(id, name string, args map[string]string) agentToolCall {
			t.Helper()
			call := agentToolCall{ID: id, Type: "function"}
			call.Function.Name = name
			encoded, _ := json.Marshal(args)
			call.Function.Arguments, _ = json.Marshal(string(encoded))
			return call
		}
		reply := agentMessage{Role: "assistant"}
		last := input.Messages[len(input.Messages)-1]
		switch last.ToolCallID {
		case "image":
			var result struct {
				ID string `json:"image_id"`
			}
			if err := json.Unmarshal([]byte(last.Content), &result); err != nil || result.ID == "" {
				t.Errorf("image generation failed: %s", last.Content)
				return
			}
			slides, _ := json.Marshal([]agentSlide{
				{Title: "Книжный клуб", Lines: []string{"Пятница после обеда"}, ImageID: result.ID},
				{Title: "Что читаем", Lines: []string{"Короткий рассказ"}},
			})
			reply.ToolCalls = []agentToolCall{addTool("deck", "file_create", map[string]string{
				"name": "Книжный клуб", "format": "pptx", "content": string(slides),
			})}
		case "deck":
			reply.Content = "Презентация готова"
		default:
			if len(input.Tools) != 2 || input.Tools[1].Function.Name != "image_generate" {
				t.Errorf("image-generation tool was not advertised: %v", input.Tools)
			}
			reply.ToolCalls = []agentToolCall{addTool("image", "image_generate", map[string]string{
				"name": "Книги", "prompt": "Книги на столе", "size": "1536x1024", "use": "pptx",
			})}
		}
		write(w, 200, map[string]any{"choices": []any{map[string]any{"message": reply}}})
	}))
	defer model.Close()

	cfg := emptyAgentConfig()
	if err := f.api.agentLoad(t.Context(), agentConfigKey(f.users[0]), &cfg); err != nil {
		t.Fatal(err)
	}
	cfg.Providers[0].URL = model.URL + "/v1"
	cfg.Providers = append(cfg.Providers, agentProvider{
		ID: "pictures", Name: "Images", Kind: "openai-compatible", URL: generator.URL + "/v1",
		Model: "fixture", APIKey: "fixture-image-key",
	})
	cfg.Agents[0].Tools = []string{"file_create", "image_generate"}
	cfg.Agents[0].ImageProviderID = "unknown"
	f.call(t, "PUT", "/tms/agents/config", f.tokens[0], cfg, 400)
	cfg.Agents[0].ImageProviderID = "pictures"
	cfg.Agents[0].ImageModel = "fixture-image"
	f.call(t, "PUT", "/tms/agents/config", f.tokens[0], cfg, 200)
	conv := f.call(t, "POST", "/tms/agents/conversations", f.tokens[0], map[string]string{"agent_id": "helper"}, 201)
	result := f.call(t, "POST", "/tms/agents/conversations/"+conv["id"].(string)+"/messages", f.tokens[0],
		map[string]string{"content": "Сделай презентацию с иллюстрацией"}, 200)
	messages := result["messages"].([]any)
	artifacts, ok := messages[len(messages)-1].(map[string]any)["artifacts"].([]any)
	if !ok || len(artifacts) != 1 || imageRequests.Load() != 1 {
		t.Fatalf("expected one deck from one image generation request, got %v and %d", messages, imageRequests.Load())
	}
	id := artifacts[0].(map[string]any)["id"].(string)
	path := "/api/v4/tms/agents/artifacts/" + id
	download := func(token string, expected int) *httptest.ResponseRecorder {
		t.Helper()
		req := httptest.NewRequest("GET", path, nil)
		req.Header.Set("Authorization", "Bearer "+token)
		res := httptest.NewRecorder()
		f.api.ServeHTTP(res, req)
		if res.Code != expected {
			t.Fatalf("deck download: got %d, want %d", res.Code, expected)
		}
		return res
	}
	download(f.tokens[1], 404)
	res := download(f.tokens[0], 200)
	if res.Header().Get("Content-Type") != agentArtifactTypes["pptx"] {
		t.Fatal(res.Header())
	}
	deck, err := zip.NewReader(bytes.NewReader(res.Body.Bytes()), int64(res.Body.Len()))
	if err != nil {
		t.Fatal(err)
	}
	part := func(name string) []byte {
		t.Helper()
		file, err := deck.Open(name)
		if err != nil {
			t.Fatal(err)
		}
		defer file.Close()
		data, err := io.ReadAll(file)
		if err != nil {
			t.Fatal(err)
		}
		return data
	}
	for name, needle := range map[string]string{
		"[Content_Types].xml":                          `Extension="png" ContentType="image/png"`,
		"ppt/theme/theme1.xml":                         `<a:theme`,
		"ppt/slideMasters/_rels/slideMaster1.xml.rels": `Target="../theme/theme1.xml"`,
		"ppt/slides/_rels/slide1.xml.rels":             `Target="../media/image1.png"`,
		"ppt/slides/slide1.xml":                        `r:embed="rId2"`,
		"ppt/slides/slide2.xml":                        "Короткий рассказ",
	} {
		if !strings.Contains(string(part(name)), needle) {
			t.Fatalf("%s is missing %s", name, needle)
		}
	}
	if !bytes.Equal(part("ppt/media/image1.png"), pngBytes.Bytes()) {
		t.Fatal("PowerPoint did not embed the generated PNG")
	}
}

func TestAgentRepairsLegacyPowerPointDownload(t *testing.T) {
	const slide = "ppt/slides/slide1.xml"
	const types = "[Content_Types].xml"
	legacy, err := agentPackage(map[string]string{
		types:                agentContentTypes(agentOverride(slide, "presentationml.slide")),
		agentSlideMasterRels: agentRels(agentRel("rId1", "slideLayout", "../slideLayouts/slideLayout1.xml")),
		slide:                `<p:sld` + agentPPTNS + `><p:cSld name="Книжный клуб"/></p:sld>`,
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	repaired, err := repairLegacyAgentPPTX(legacy)
	if err != nil {
		t.Fatal(err)
	}
	read := func(data []byte, part string) string {
		t.Helper()
		archive, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
		if err != nil {
			t.Fatal(err)
		}
		content, err := fs.ReadFile(archive, part)
		if err != nil {
			t.Fatal(err)
		}
		return string(content)
	}
	if read(legacy, slide) != read(repaired, slide) ||
		!strings.Contains(read(repaired, types), agentPPTXThemePart) ||
		!strings.Contains(read(repaired, agentSlideMasterRels), "theme/theme1.xml") ||
		!strings.Contains(read(repaired, agentPPTXThemePart), "<a:theme") {
		t.Fatal("repair must add the theme and preserve original slides")
	}
	alreadyRepaired, err := repairLegacyAgentPPTX(repaired)
	if err != nil || !bytes.Equal(repaired, alreadyRepaired) {
		t.Fatal("a themed package should be returned as-is", err)
	}
}
