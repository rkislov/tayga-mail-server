package httpapi

import (
	"fmt"
	"strings"
)

func readComparePPTX(pkg *comparePackage, doc *structuredDocument) error {
	presentation, err := pkg.xml("ppt/presentation.xml")
	if err != nil {
		return err
	}
	if presentation.name.Local != "presentation" {
		return fmt.Errorf("invalid presentation")
	}
	rels, err := pkg.relationships("ppt/presentation.xml")
	if err != nil {
		return err
	}
	for index, slide := range presentation.child("sldIdLst").all("sldId") {
		rel, ok := rels[slide.relationshipID()]
		if !ok || rel.external || !strings.HasSuffix(rel.kind, "/slide") {
			return fmt.Errorf("invalid slide relationship")
		}
		body, e := pkg.xml(rel.target)
		if e != nil {
			return e
		}
		if body.name.Local != "sld" {
			return fmt.Errorf("invalid slide")
		}
		paragraphs := []string{}
		for _, paragraph := range body.descendants("p") {
			if !strings.Contains(paragraph.name.Space, "drawingml/") {
				continue
			}
			text := drawingCompareText(paragraph)
			if text != "" {
				paragraphs = append(paragraphs, text)
			}
		}
		links, e := pkg.relationships(rel.target)
		if e != nil {
			return e
		}
		images := []string{}
		// Resolve only images actually used by this slide, never unused ZIP files.
		imageReferences := append(body.descendants("blip"), body.descendants("svgBlip")...)
		for _, image := range imageReferences {
			for _, attr := range image.attrs {
				if attr.Name.Local != "embed" && attr.Name.Local != "link" {
					continue
				}
				reference, ok := links[attr.Value]
				if !ok || !strings.HasSuffix(reference.kind, "/image") {
					return fmt.Errorf("invalid slide image relationship")
				}
				if reference.external {
					images = append(images, "external: "+reference.target)
					continue
				}
				hash, e := compareImage(pkg, reference.target)
				if e != nil {
					return e
				}
				images = append(images, hash)
			}
		}
		title := body.child("cSld").attr("name")
		if e = compareSlide(doc, slide.plainAttr("id"), title, strings.Join(paragraphs, "\n"), images, index+1, body.attr("show") == "0" || body.attr("show") == "false"); e != nil {
			return e
		}
	}
	return nil
}

func drawingCompareText(node *compareNode) string {
	if strings.Contains(node.name.Space, "drawingml/") {
		switch node.name.Local {
		case "t":
			return node.text()
		case "br":
			return "\n"
		case "tab":
			return "\t"
		}
	}
	var out strings.Builder
	for _, child := range node.children {
		out.WriteString(drawingCompareText(child))
	}
	return out.String()
}
