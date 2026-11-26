package mattermost

import (
	"archive/zip"
	"bytes"
	"io/fs"
	"strings"
)

// Previously saved presentations from Tayga used a slide master without a
// theme. Repair that exact package structure when the owner downloads it.
func repairLegacyAgentPPTX(data []byte) ([]byte, error) {
	const contentTypes = "[Content_Types].xml"
	source, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, err
	}
	for _, part := range source.File {
		if strings.HasPrefix(part.Name, "ppt/theme/") {
			return data, nil
		}
	}
	master, err := fs.ReadFile(source, agentSlideMasterRels)
	if err != nil || string(master) != agentOOXMLHeader+agentRels(agentRel("rId1", "slideLayout", "../slideLayouts/slideLayout1.xml")) {
		return data, nil
	}
	declarations, err := fs.ReadFile(source, contentTypes)
	if err != nil {
		return nil, err
	}
	types, packageXML := strings.CutPrefix(string(declarations), agentOOXMLHeader)
	types, endTypes := strings.CutSuffix(types, "</Types>")
	if !packageXML || !endTypes {
		return data, nil
	}
	var buf bytes.Buffer
	result := zip.NewWriter(&buf)
	for _, part := range source.File {
		switch part.Name {
		case agentSlideMasterRels:
			err = writeAgentXML(result, part.Name, agentMasterRelationships())
		case contentTypes:
			err = writeAgentXML(result, part.Name, types+agentOverride(agentPPTXThemePart, "theme")+"</Types>")
		default:
			err = result.Copy(part)
		}
		if err != nil {
			return nil, err
		}
	}
	if err = writeAgentXML(result, agentPPTXThemePart, agentPPTXTheme); err != nil {
		return nil, err
	}
	if err = result.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
