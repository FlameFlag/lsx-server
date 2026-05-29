package discord

import (
	"bytes"
	"encoding/json"
	"io"
	"mime/multipart"
	"os"
	urlpath "path"
	"strings"
	"time"

	"lt2_reverse/lsx_server_go/internal/lsx"
	"lt2_reverse/lsx_server_go/internal/strutil"
)

type payload struct {
	Username        string          `json:"username,omitempty"`
	AvatarURL       string          `json:"avatar_url,omitempty"`
	Embeds          []embed         `json:"embeds,omitempty"`
	AllowedMentions allowedMentions `json:"allowed_mentions"`
	Attachments     []attachment    `json:"attachments,omitempty"`
}

type allowedMentions struct {
	Parse []string `json:"parse"`
}

type embed struct {
	Title       string  `json:"title"`
	Description string  `json:"description,omitempty"`
	Color       int     `json:"color"`
	Author      *author `json:"author,omitempty"`
	Thumbnail   *image  `json:"thumbnail,omitempty"`
	Fields      []field `json:"fields,omitempty"`
	Footer      *footer `json:"footer,omitempty"`
	Timestamp   string  `json:"timestamp"`
}

type author struct {
	Name string `json:"name"`
}

type footer struct {
	Text string `json:"text"`
}

type image struct {
	URL string `json:"url"`
}

type field struct {
	Name   string `json:"name"`
	Value  string `json:"value"`
	Inline bool   `json:"inline"`
}

type attachment struct {
	ID          int    `json:"id"`
	Filename    string `json:"filename"`
	Description string `json:"description,omitempty"`
}

type file struct {
	path     string
	filename string
	data     []byte
}

func (n *Notifier) payload(ev lsx.Event) (payload, *file, error) {
	attachmentFile := n.attachment()
	rendered := render(ev)
	payload := payload{
		Username: "LT2 LSX",
		Embeds: []embed{{
			Title:       rendered.title,
			Description: rendered.description,
			Color:       colorFor(ev),
			Author:      &author{Name: "Lemonade Tycoon 2 Stock Exchange"},
			Thumbnail:   thumbnail(attachmentFile),
			Fields:      rendered.fields,
			Footer:      &footer{Text: footerText(ev)},
			Timestamp:   ev.Time.UTC().Format(time.RFC3339),
		}},
		AllowedMentions: allowedMentions{Parse: []string{}},
	}
	if attachmentFile != nil {
		payload.Attachments = []attachment{{
			ID:          0,
			Filename:    attachmentFile.filename,
			Description: "Recovered Lemonade Tycoon 2 icon",
		}}
	}
	return payload, attachmentFile, nil
}

func (n *Notifier) attachment() *file {
	if n == nil || n.iconPath == "" {
		return nil
	}
	if n.iconPath == "embedded" && len(n.embeddedIcon) > 0 {
		return &file{filename: strutil.FirstNonEmpty(n.embeddedIconName, "discord-icon.png"), data: n.embeddedIcon}
	}
	info, err := os.Stat(n.iconPath)
	if err != nil || info.IsDir() {
		return nil
	}
	return &file{
		path:     n.iconPath,
		filename: attachmentName(n.iconPath, n.embeddedIconName),
	}
}

func thumbnail(file *file) *image {
	if file == nil {
		return nil
	}
	return &image{URL: "attachment://" + file.filename}
}

func encodePayload(payload payload, attachment *file) (io.Reader, string, error) {
	if attachment == nil {
		body, err := json.Marshal(payload)
		if err != nil {
			return nil, "", err
		}
		return bytes.NewReader(body), "application/json", nil
	}

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	payloadPart, err := writer.CreateFormField("payload_json")
	if err != nil {
		return nil, "", err
	}
	if err := json.NewEncoder(payloadPart).Encode(payload); err != nil {
		return nil, "", err
	}

	filePart, err := writer.CreateFormFile("files[0]", attachment.filename)
	if err != nil {
		return nil, "", err
	}
	if len(attachment.data) > 0 {
		if _, err := filePart.Write(attachment.data); err != nil {
			return nil, "", err
		}
	} else {
		file, err := os.Open(attachment.path)
		if err != nil {
			return nil, "", err
		}
		defer func() { _ = file.Close() }()
		if _, err := io.Copy(filePart, file); err != nil {
			return nil, "", err
		}
	}
	if err := writer.Close(); err != nil {
		return nil, "", err
	}
	return &body, writer.FormDataContentType(), nil
}

func attachmentName(filePath, fallback string) string {
	filePath = strings.ReplaceAll(filePath, "\\", "/")
	name := urlpath.Base(filePath)
	if strings.TrimSpace(name) == "" {
		return strutil.FirstNonEmpty(fallback, "discord-icon.png")
	}
	return name
}
