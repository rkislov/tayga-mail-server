package httpapi

import "github.com/tayga/tms/internal/mailcompose"

const maxEmbeddedImageBytes = mailcompose.MaxEmbeddedImageBytes
const maxEmbeddedImagesBytes = mailcompose.MaxEmbeddedImagesBytes
const maxWelcomeHTMLBytes = mailcompose.MaxWelcomeHTMLBytes

type inlineMailImage = mailcompose.InlineImage

func decodeEmbeddedMailImage(value string) (inlineMailImage, error) {
	return mailcompose.DecodeEmbeddedImage(value)
}
func prepareInlineMailImages(source string) (string, []inlineMailImage, error) {
	return mailcompose.PrepareInlineImages(source)
}
func attachInlineMailImages(raw []byte, images []inlineMailImage) ([]byte, error) {
	return mailcompose.AttachInlineImages(raw, images)
}
