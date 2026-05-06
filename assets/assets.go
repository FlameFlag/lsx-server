package assets

import "embed"

//go:generate go run ../tools/generate_findings_content
//go:generate go run ../tools/download_findings_shiki.go
//go:embed admin/*.css admin/dashboard/*.css admin/login/*.css admin/*.avif legacy/*.css project/css/*.css project/*.avif project/*.png project/fonts/* project/docs/*.html.tmpl project/docs/*.js project/findings/*.html.tmpl project/findings/*.js project/findings/vendor/shiki
var FS embed.FS
