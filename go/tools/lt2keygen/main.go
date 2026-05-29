package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"lt2_reverse/lsx_server_go/internal/lsx/keygen"
	lt2ui "lt2_reverse/lsx_server_go/tools/lt2keygen/internal/ui"
)

const note = "Naturally-valid Armadillo ShortV3 signed key. The key is bound to the registration name: use this exact name/key pair in Lemonade2.exe REGISTER."

type response struct {
	RegistrationName string `json:"registration_name"`
	ActivationKey    string `json:"activation_key"`
	KeyFormat        string `json:"key_format"`
	Note             string `json:"note"`
}

func generateResponse(name string) (response, error) {
	pair, err := keygen.Generate(name)
	if err != nil {
		return response{}, err
	}
	return response{
		RegistrationName: pair.RegistrationName,
		ActivationKey:    pair.ActivationKey,
		KeyFormat:        pair.Format,
		Note:             note,
	}, nil
}

func main() {
	os.Exit(run(os.Args[1:]))
}

func run(args []string) int {
	if len(args) == 0 {
		return lt2ui.Run("auto", uiGenerate)
	}

	fs := flag.NewFlagSet("lt2keygen", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	nameFlag := fs.String("name", "", "registration name; random if omitted")
	jsonOutput := fs.Bool("json", false, "write JSON output")
	uiMode := fs.String("ui", "cli", "interface to launch: cli, auto, gtk, swiftui, or windows")
	fs.Usage = func() {
		fmt.Fprintf(fs.Output(), "Usage: %s [flags] [registration-name]\n", fs.Name())
		fs.PrintDefaults()
	}

	if err := fs.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return 0
		}
		return 2
	}
	if fs.NArg() > 1 {
		fs.Usage()
		return 2
	}
	if *uiMode != "" && *uiMode != "cli" {
		return lt2ui.Run(*uiMode, uiGenerate)
	}

	name := *nameFlag
	if fs.NArg() == 1 {
		if name != "" {
			fmt.Fprintln(os.Stderr, "use either -name or positional registration-name, not both")
			return 2
		}
		name = fs.Arg(0)
	}

	out, err := generateResponse(name)
	if err != nil {
		fmt.Fprintf(os.Stderr, "key generation failed: %v\n", err)
		return 1
	}
	if *jsonOutput {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(out); err != nil {
			fmt.Fprintf(os.Stderr, "write JSON: %v\n", err)
			return 1
		}
		return 0
	}

	fmt.Printf("Registration name: %s\n", out.RegistrationName)
	fmt.Printf("Activation key:    %s\n", out.ActivationKey)
	fmt.Printf("Key format:        %s\n", out.KeyFormat)
	fmt.Printf("Note:              %s\n", out.Note)
	return 0
}

func uiGenerate(name string) (lt2ui.Response, error) {
	out, err := generateResponse(name)
	if err != nil {
		return lt2ui.Response{}, err
	}
	return lt2ui.Response{
		RegistrationName: out.RegistrationName,
		ActivationKey:    out.ActivationKey,
		KeyFormat:        out.KeyFormat,
		Note:             out.Note,
	}, nil
}
