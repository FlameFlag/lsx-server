package main

import (
	"fmt"
	"io"
	"os"
	"strings"
	"text/tabwriter"
)

const knownInstallerMD5 = "e909603b19884540048e4ac81dfcf044"

type compression string

const (
	compressionRaw   compression = "raw"
	compressionBzip2 compression = "bzip2"
	compressionZlib  compression = "zlib"
)

type archivePart struct {
	algo   compression
	offset int64
	length int64
}

type installEntry struct {
	name  string
	size  int64
	md5   string
	mode  os.FileMode
	parts []archivePart
}

var installEntries = []installEntry{
	{
		name: "Uninstal.exe",
		size: 74717,
		md5:  "42cf4866276b1186ba487d3289bc6fb8",
		mode: 0o755,
		parts: []archivePart{
			{algo: compressionZlib, offset: 0x30AB1, length: 0x6A2B},
			{algo: compressionRaw, offset: 0x37B99, length: 0x3DD},
		},
	},
	{
		name: "fmod.dll",
		size: 130560,
		md5:  "67c0394f64446852f1438492b97b1173",
		mode: 0o644,
		parts: []archivePart{
			{algo: compressionZlib, offset: 0x37F77, length: 0x1DEBF},
		},
	},
	{
		name: "Lemonade2.exe",
		size: 765952,
		md5:  "3a53293dca5e9e84b0050758dffe41a5",
		mode: 0o755,
		parts: []archivePart{
			{algo: compressionZlib, offset: 0x55E37, length: 0xA8C14},
		},
	},
	{
		name: "Lemonade2.rb",
		size: 27886734,
		md5:  "94782e5a66923ee74e9189ee1cc2e47c",
		mode: 0o644,
		parts: []archivePart{
			{algo: compressionBzip2, offset: 0xFEA4C, length: 0x7B2C67},
		},
	},
	{
		name: "Lsx\\CheckConnection.html",
		size: 1036,
		md5:  "0433925f8ebaf6048c8429717551fca0",
		mode: 0o644,
		parts: []archivePart{
			{algo: compressionZlib, offset: 0x8B16B4, length: 0x20A},
		},
	},
	{
		name: "Lsx\\NoConnection.gif",
		size: 702,
		md5:  "f6245e84e0ff4e077ae29bcde8bb6794",
		mode: 0o644,
		parts: []archivePart{
			{algo: compressionZlib, offset: 0x8B18BF, length: 0x2C9},
		},
	},
	{
		name: "Lsx\\Thumbs.db",
		size: 3072,
		md5:  "2698f032aa031ce4415481e2feacddd0",
		mode: 0o644,
		parts: []archivePart{
			{algo: compressionZlib, offset: 0x8B1B89, length: 0x375},
		},
	},
	{
		name: "options.dat",
		size: 100,
		md5:  "416f2f312eade1a804ee530143ad1666",
		mode: 0o644,
		parts: []archivePart{
			{algo: compressionZlib, offset: 0x8B1EFF, length: 0x24},
		},
	},
	{
		name: "TeneonIERelease.dll",
		size: 352369,
		md5:  "2f8cbe06c420735617fbd893aa80f5fc",
		mode: 0o644,
		parts: []archivePart{
			{algo: compressionBzip2, offset: 0x8B1F24, length: 0x2A2A3},
		},
	},
}

func printManifest(w io.Writer) {
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	_, _ = fmt.Fprintln(tw, "PATH\tPARTS\tSIZE\tMD5")
	for _, entry := range installEntries {
		_, _ = fmt.Fprintf(tw, "%s\t%s\t%d\t%s\n",
			entry.name,
			describeParts(entry.parts),
			entry.size,
			entry.md5,
		)
	}
	_ = tw.Flush()
}

func describeParts(parts []archivePart) string {
	descriptions := make([]string, 0, len(parts))
	for _, part := range parts {
		descriptions = append(descriptions, fmt.Sprintf("%s@0x%X+%d", part.algo, part.offset, part.length))
	}
	return strings.Join(descriptions, ",")
}
