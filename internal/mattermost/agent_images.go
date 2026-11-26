package mattermost

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"image/png"
	"strings"
	"time"
)

const defaultAgentImageModel = "gpt-image-2.6-flare"
const agentImageLimit = 8 << 20
const agentImageResponseLimit = 12 << 20

type agentGeneratedImage struct {
	name          string
	data          []byte
	width, height int
}

func agentImageProvider(config agentConfiguration, agent agentDefinition) *agentProvider {
	if !agentHasTool(agent, "image_generate") {
		return nil
	}
	for i := range config.Providers {
		provider := &config.Providers[i]
		if provider.ID == agent.ImageProviderID && (provider.Kind == "openai" || provider.Kind == "openai-compatible") &&
			validAgentProviderEndpoint(*provider) {
			return provider
		}
	}
	return nil
}

func (f *agentFiles) generateImage(ctx context.Context, provider agentProvider, model string, args map[string]string) (any, error) {
	if f.imageAttempts >= 3 {
		return nil, errors.New("можно запросить до трёх изображений за один ответ")
	}
	usage := args["use"]
	if usage != "pptx" && usage != "download" {
		return nil, errors.New("укажите use=pptx для слайда или use=download для отдельного PNG")
	}
	if usage == "download" && len(f.outputs) >= 3 {
		return nil, errors.New("в ответе уже три файла")
	}
	prompt := strings.TrimSpace(args["prompt"])
	if prompt == "" || len(prompt) > 12000 {
		return nil, errors.New("укажите описание изображения до 12 КБ")
	}
	name, err := agentArtifactName(args["name"], "png")
	if err != nil {
		return nil, err
	}
	size := args["size"]
	switch size {
	case "":
		size = "1024x1024"
		if usage == "pptx" {
			size = "1536x1024"
		}
	case "1024x1024", "1536x1024", "1024x1536":
	default:
		return nil, errors.New("доступные размеры: 1024x1024, 1536x1024, 1024x1536")
	}
	if model = strings.TrimSpace(model); model == "" {
		model = defaultAgentImageModel
	}
	client, err := newAgentProviderClient(provider, 120*time.Second)
	if err != nil {
		return nil, errors.New("проверьте подключение к провайдеру изображений")
	}
	if client.Transport != nil {
		defer client.CloseIdleConnections()
	}
	f.imageAttempts++
	endpoint := strings.TrimRight(provider.URL, "/") + "/images/generations"
	payload := map[string]any{
		"model": model, "prompt": prompt, "size": size, "quality": "medium",
		"output_format": "png", "n": 1,
	}
	raw, err := sendAgentProviderRequest(ctx, provider, endpoint, payload, client, agentImageResponseLimit)
	if err != nil {
		var rejection agentProviderError
		if errors.As(err, &rejection) {
			return nil, errors.New(rejection.message())
		}
		return nil, errors.New("провайдер изображений не ответил")
	}
	var response struct {
		Data []struct {
			Base64 string `json:"b64_json"`
		} `json:"data"`
	}
	invalid := errors.New("провайдер должен вернуть одно корректное PNG в поле b64_json, до 8 МиБ")
	if json.Unmarshal(raw, &response) != nil || len(response.Data) != 1 {
		return nil, invalid
	}
	encoded := response.Data[0].Base64
	if encoded == "" || len(encoded) > base64.StdEncoding.EncodedLen(agentImageLimit) {
		return nil, invalid
	}
	data, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil || len(data) > agentImageLimit {
		return nil, invalid
	}
	dimensions, err := png.DecodeConfig(bytes.NewReader(data))
	if err != nil || dimensions.Width < 1 || dimensions.Height < 1 || dimensions.Width > 2048 || dimensions.Height > 2048 {
		return nil, invalid
	}
	if _, err := png.Decode(bytes.NewReader(data)); err != nil {
		return nil, invalid
	}
	if f.images == nil {
		f.images = make(map[string]agentGeneratedImage)
	}
	id := agentID()
	f.images[id] = agentGeneratedImage{name: name, data: data, width: dimensions.Width, height: dimensions.Height}
	result := map[string]any{"image_id": id, "name": name, "width": dimensions.Width, "height": dimensions.Height}
	if usage == "download" {
		result["file"] = f.save(name, "image/png", data)
	}
	return result, nil
}
