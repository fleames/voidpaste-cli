package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"golang.org/x/term"
)

func root(args []string) error {
	if len(args) == 0 {
		printUsage(os.Stdout)
		return nil
	}
	switch args[0] {
	case "help", "-h", "--help":
		printUsage(os.Stdout)
		return nil
	case "version", "--version":
		fmt.Println("voidpaste 0.2.0")
		return nil
	case "auth":
		return cmdAuth(args[1:])
	case "status", "health":
		return cmdStatus(args[1:])
	case "whoami":
		return cmdWhoami(args)
	case "paste":
		return cmdPaste(args[1:])
	case "create":
		return cmdPasteCreate(args[1:])
	case "get":
		return cmdPasteGet(args[1:])
	case "raw":
		return cmdPasteRaw(args[1:])
	case "download":
		return cmdPasteDownload(args[1:])
	case "list":
		return cmdPasteList(args[1:])
	case "delete", "rm":
		return cmdPasteDelete(args[1:])
	case "collection", "collections":
		return cmdCollection(args[1:])
	case "versions":
		return cmdVersions(args[1:])
	default:
		return fmt.Errorf("unknown command %q (try `voidpaste help`)", args[0])
	}
}

func printUsage(w io.Writer) {
	fmt.Fprint(w, `voidpaste — CLI for https://voidpaste.com

Usage:
  voidpaste <command> [flags]

Auth:
  voidpaste auth login --key vp_live_…   Store API key in ~/.config/voidpaste
  voidpaste auth logout                  Remove stored API key
  voidpaste auth whoami                  Show account for the current key
  voidpaste whoami                       Alias of auth whoami

Pastes:
  voidpaste create [file] [flags]        Create paste (stdin if no file / --stdin)
  voidpaste get <id>                     Fetch paste JSON (metadata + content)
  voidpaste raw <id>                     Fetch raw body
  voidpaste download <id> [-o file]      Download attachment body
  voidpaste list [--limit N]             List your pastes (requires API key)
  voidpaste delete <id>                  Delete a paste you own

  voidpaste paste <subcommand>           Same as the shortcuts above

Collections (API key required):
  voidpaste collection list
  voidpaste collection create --name NAME [--visibility private]
  voidpaste collection get <id>
  voidpaste collection delete <id>
  voidpaste collection add <id> <pasteId>
  voidpaste collection remove <id> <pasteId>

Versions (API key required):
  voidpaste versions list <pasteId>
  voidpaste versions get <pasteId> <n>
  voidpaste versions restore <pasteId> <n>

Status:
  voidpaste status                       API /health (+ whoami when keyed)

Global flags (after the command):
  --api URL     API origin (default https://voidpaste.com; or env VP_API)
  --key KEY     API key for this invocation (or env VP_API_KEY / VP_TOKEN)
  --json        Print raw JSON where applicable

Install:
  go install github.com/fleames/voidpaste-cli/cmd/voidpaste@latest

Config file: $XDG_CONFIG_HOME/voidpaste/config.json (or %%AppData%%\voidpaste on Windows)
Env: VP_API_KEY, VP_API, VP_TOKEN (alias), VP_CONFIG_DIR
`)
}

func parseGlobal(fs *flag.FlagSet, args []string) (api, key string, jsonOut bool, rest []string, err error) {
	fs.StringVar(&api, "api", "", "API origin (default https://voidpaste.com)")
	fs.StringVar(&key, "key", "", "API key (Bearer vp_live_…)")
	fs.BoolVar(&jsonOut, "json", false, "print JSON")
	rest, err = parseFlagsAllowInterspersed(fs, args)
	return
}

// parseFlagsAllowInterspersed accepts `cmd <id> --api URL` as well as `cmd --api URL <id>`.
func parseFlagsAllowInterspersed(fs *flag.FlagSet, args []string) ([]string, error) {
	type boolFlag interface {
		IsBoolFlag() bool
	}
	var flags, positionals []string
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch {
		case a == "--":
			positionals = append(positionals, args[i+1:]...)
			return positionalsAfterParse(fs, flags, positionals)
		case strings.HasPrefix(a, "-"):
			flags = append(flags, a)
			if strings.Contains(a, "=") {
				continue
			}
			name := strings.TrimLeft(a, "-")
			if f := fs.Lookup(name); f != nil {
				if bf, ok := f.Value.(boolFlag); ok && bf.IsBoolFlag() {
					continue
				}
			}
			if i+1 < len(args) && !strings.HasPrefix(args[i+1], "-") {
				i++
				flags = append(flags, args[i])
			}
		default:
			positionals = append(positionals, a)
		}
	}
	return positionalsAfterParse(fs, flags, positionals)
}

func positionalsAfterParse(fs *flag.FlagSet, flags, positionals []string) ([]string, error) {
	if err := fs.Parse(flags); err != nil {
		return nil, err
	}
	return positionals, nil
}

func cmdAuth(args []string) error {
	if len(args) == 0 {
		return errors.New("usage: voidpaste auth <login|logout|whoami>")
	}
	switch args[0] {
	case "login":
		return cmdAuthLogin(args[1:])
	case "logout":
		return cmdAuthLogout(args[1:])
	case "whoami":
		return cmdWhoami(args[1:])
	default:
		return fmt.Errorf("unknown auth command %q", args[0])
	}
}

func cmdAuthLogin(args []string) error {
	fs := flag.NewFlagSet("auth login", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	var api, key string
	var fromStdin bool
	fs.StringVar(&api, "api", "", "API origin to store")
	fs.StringVar(&key, "key", "", "API key secret")
	fs.BoolVar(&fromStdin, "stdin", false, "read API key from stdin")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fromStdin {
		b, err := io.ReadAll(os.Stdin)
		if err != nil {
			return err
		}
		key = strings.TrimSpace(string(b))
	}
	if key == "" {
		fmt.Fprint(os.Stderr, "API key: ")
		b, err := term.ReadPassword(int(os.Stdin.Fd()))
		fmt.Fprintln(os.Stderr)
		if err != nil {
			return err
		}
		key = strings.TrimSpace(string(b))
	}
	if key == "" {
		return errors.New("--key is required (vp_live_… from Dashboard → API Keys)")
	}
	if !strings.HasPrefix(key, "vp_live_") && !strings.HasPrefix(key, "vp_test_") {
		return errors.New("key must start with vp_live_ or vp_test_")
	}
	cfg, err := loadConfig()
	if err != nil {
		return err
	}
	cfg.APIKey = key
	if api != "" {
		cfg.APIBase = strings.TrimRight(api, "/")
	}
	if cfg.APIBase == "" {
		cfg.APIBase = defaultAPIBase
	}
	// Verify key before saving.
	client := NewClient(cfg)
	if _, err := client.Me(context.Background()); err != nil {
		return fmt.Errorf("key rejected by %s: %w", cfg.APIBase, err)
	}
	if err := saveConfig(cfg); err != nil {
		return err
	}
	path, _ := configPath()
	fmt.Printf("Logged in. Key stored in %s\n", path)
	fmt.Printf("API: %s\n", cfg.APIBase)
	return nil
}

func cmdAuthLogout(args []string) error {
	fs := flag.NewFlagSet("auth logout", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	if err := fs.Parse(args); err != nil {
		return err
	}
	cfg, err := loadConfig()
	if err != nil {
		return err
	}
	cfg.APIKey = ""
	if err := saveConfig(cfg); err != nil {
		return err
	}
	fmt.Println("Logged out (API key removed from config).")
	return nil
}

func cmdWhoami(args []string) error {
	fs := flag.NewFlagSet("whoami", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	api, key, jsonOut, _, err := parseGlobal(fs, args)
	if err != nil {
		return err
	}
	cfg, err := resolve(api, key)
	if err != nil {
		return err
	}
	if err := requireKey(cfg); err != nil {
		return err
	}
	out, err := NewClient(cfg).Me(context.Background())
	if err != nil {
		return err
	}
	if jsonOut {
		return printJSON(out)
	}
	user, _ := out["user"].(map[string]any)
	if user == nil {
		return printJSON(out)
	}
	fmt.Printf("email:   %v\n", user["email"])
	fmt.Printf("id:      %v\n", user["id"])
	fmt.Printf("role:    %v\n", user["role"])
	fmt.Printf("plan:    %v\n", user["plan"])
	fmt.Printf("api:     %s\n", cfg.APIBase)
	return nil
}

func cmdStatus(args []string) error {
	fs := flag.NewFlagSet("status", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	api, key, jsonOut, _, err := parseGlobal(fs, args)
	if err != nil {
		return err
	}
	cfg, err := resolve(api, key)
	if err != nil {
		return err
	}
	client := NewClient(cfg)
	health, err := client.Health(context.Background())
	if err != nil {
		return err
	}
	payload := map[string]any{
		"api":    cfg.APIBase,
		"health": health,
	}
	if cfg.APIKey != "" {
		if me, err := client.Me(context.Background()); err == nil {
			payload["user"] = me["user"]
		} else {
			payload["auth_error"] = err.Error()
		}
	}
	if jsonOut {
		return printJSON(payload)
	}
	fmt.Printf("api:     %s\n", cfg.APIBase)
	fmt.Printf("health:  %v\n", health["status"])
	if u, ok := payload["user"].(map[string]any); ok {
		fmt.Printf("user:    %v\n", u["email"])
	} else if ae, ok := payload["auth_error"]; ok {
		fmt.Printf("auth:    %v\n", ae)
	} else {
		fmt.Println("auth:    (no API key)")
	}
	return nil
}

func cmdPaste(args []string) error {
	if len(args) == 0 {
		return errors.New("usage: voidpaste paste <create|get|raw|download|list|delete>")
	}
	switch args[0] {
	case "create":
		return cmdPasteCreate(args[1:])
	case "get":
		return cmdPasteGet(args[1:])
	case "raw":
		return cmdPasteRaw(args[1:])
	case "download":
		return cmdPasteDownload(args[1:])
	case "list":
		return cmdPasteList(args[1:])
	case "delete", "rm":
		return cmdPasteDelete(args[1:])
	default:
		return fmt.Errorf("unknown paste command %q", args[0])
	}
}

func cmdPasteCreate(args []string) error {
	fs := flag.NewFlagSet("create", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	var (
		api, key, title, language, visibility, expiration, password string
		burn, useStdin, jsonOut                                     bool
	)
	fs.StringVar(&api, "api", "", "")
	fs.StringVar(&key, "key", "", "")
	fs.StringVar(&title, "title", "", "paste title")
	fs.StringVar(&language, "language", "", "syntax language (e.g. go, python)")
	fs.StringVar(&language, "lang", "", "alias of --language")
	fs.StringVar(&visibility, "visibility", "unlisted", "public|unlisted|private|password")
	fs.StringVar(&expiration, "expiration", "", "e.g. 1h, 1d, 1w, 1M, never")
	fs.StringVar(&expiration, "expires", "", "alias of --expiration")
	fs.StringVar(&password, "password", "", "paste password (requires --visibility=password)")
	fs.BoolVar(&burn, "burn", false, "burn after reading")
	fs.BoolVar(&useStdin, "stdin", false, "read content from stdin")
	fs.BoolVar(&jsonOut, "json", false, "print full JSON response")
	positionals, err := parseFlagsAllowInterspersed(fs, args)
	if err != nil {
		return err
	}
	cfg, err := resolve(api, key)
	if err != nil {
		return err
	}
	// Anonymous create is allowed; key optional unless private.
	content, err := readContent(positionals, useStdin)
	if err != nil {
		return err
	}
	if visibility == "private" || visibility == "password" {
		if err := requireKey(cfg); err != nil {
			return err
		}
	}
	in := CreatePasteInput{
		Title:            title,
		Language:         language,
		Visibility:       visibility,
		Password:         password,
		Expiration:       expiration,
		BurnAfterReading: burn,
		Content:          content,
	}
	out, err := NewClient(cfg).CreatePaste(context.Background(), in)
	if err != nil {
		return err
	}
	if jsonOut {
		return printJSON(out)
	}
	paste, _ := out["paste"].(map[string]any)
	if paste == nil {
		return printJSON(out)
	}
	id, _ := paste["id"].(string)
	fmt.Println(id)
	fmt.Printf("%s/p/%s\n", strings.TrimRight(cfg.APIBase, "/"), id)
	if warnings, ok := out["warnings"]; ok && warnings != nil {
		fmt.Fprintf(os.Stderr, "warnings: %v\n", warnings)
	}
	return nil
}

func readContent(positional []string, forceStdin bool) (string, error) {
	if forceStdin || len(positional) == 0 {
		if len(positional) > 0 {
			return "", errors.New("pass either a file path or --stdin, not both")
		}
		if !forceStdin && term.IsTerminal(int(os.Stdin.Fd())) {
			return "", errors.New("no file given and stdin is a terminal; pass a path or pipe content / --stdin")
		}
		b, err := io.ReadAll(io.LimitReader(os.Stdin, 2<<20))
		if err != nil {
			return "", err
		}
		if len(b) == 0 {
			return "", errors.New("empty content")
		}
		return string(b), nil
	}
	if len(positional) != 1 {
		return "", errors.New("create accepts at most one file path")
	}
	b, err := os.ReadFile(positional[0])
	if err != nil {
		return "", err
	}
	if len(b) == 0 {
		return "", errors.New("empty file")
	}
	if len(b) > 1<<20 {
		return "", errors.New("file exceeds 1 MB paste limit")
	}
	return string(b), nil
}

func cmdPasteGet(args []string) error {
	fs := flag.NewFlagSet("get", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	var api, key, password string
	var jsonOut bool
	fs.StringVar(&api, "api", "", "")
	fs.StringVar(&key, "key", "", "")
	fs.StringVar(&password, "password", "", "paste password")
	fs.BoolVar(&jsonOut, "json", false, "print JSON (default)")
	positionals, err := parseFlagsAllowInterspersed(fs, args)
	if err != nil {
		return err
	}
	if len(positionals) != 1 {
		return errors.New("usage: voidpaste get <id>")
	}
	cfg, err := resolve(api, key)
	if err != nil {
		return err
	}
	out, err := NewClient(cfg).GetPaste(context.Background(), positionals[0], password)
	if err != nil {
		return err
	}
	_ = jsonOut
	return printJSON(out)
}

func cmdPasteRaw(args []string) error {
	fs := flag.NewFlagSet("raw", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	var api, key, password string
	fs.StringVar(&api, "api", "", "")
	fs.StringVar(&key, "key", "", "")
	fs.StringVar(&password, "password", "", "paste password")
	positionals, err := parseFlagsAllowInterspersed(fs, args)
	if err != nil {
		return err
	}
	if len(positionals) != 1 {
		return errors.New("usage: voidpaste raw <id>")
	}
	cfg, err := resolve(api, key)
	if err != nil {
		return err
	}
	data, err := NewClient(cfg).RawPaste(context.Background(), positionals[0], password)
	if err != nil {
		return err
	}
	_, err = os.Stdout.Write(data)
	return err
}

func cmdPasteDownload(args []string) error {
	fs := flag.NewFlagSet("download", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	var api, key, password, outPath string
	fs.StringVar(&api, "api", "", "")
	fs.StringVar(&key, "key", "", "")
	fs.StringVar(&password, "password", "", "paste password")
	fs.StringVar(&outPath, "o", "", "output file (default: stdout or suggested name)")
	fs.StringVar(&outPath, "out", "", "alias of -o")
	positionals, err := parseFlagsAllowInterspersed(fs, args)
	if err != nil {
		return err
	}
	if len(positionals) != 1 {
		return errors.New("usage: voidpaste download <id> [-o file]")
	}
	cfg, err := resolve(api, key)
	if err != nil {
		return err
	}
	data, name, err := NewClient(cfg).DownloadPaste(context.Background(), positionals[0], password)
	if err != nil {
		return err
	}
	if outPath == "" {
		if term.IsTerminal(int(os.Stdout.Fd())) && name != "" {
			outPath = name
		} else {
			_, err = os.Stdout.Write(data)
			return err
		}
	}
	if err := os.WriteFile(outPath, data, 0o644); err != nil {
		return err
	}
	fmt.Fprintf(os.Stderr, "wrote %s (%d bytes)\n", outPath, len(data))
	return nil
}

func cmdPasteList(args []string) error {
	fs := flag.NewFlagSet("list", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	var api, key string
	var limit, offset int
	var jsonOut bool
	fs.StringVar(&api, "api", "", "")
	fs.StringVar(&key, "key", "", "")
	fs.IntVar(&limit, "limit", 50, "max pastes")
	fs.IntVar(&offset, "offset", 0, "offset")
	fs.BoolVar(&jsonOut, "json", false, "print JSON")
	if _, err := parseFlagsAllowInterspersed(fs, args); err != nil {
		return err
	}
	cfg, err := resolve(api, key)
	if err != nil {
		return err
	}
	if err := requireKey(cfg); err != nil {
		return err
	}
	out, err := NewClient(cfg).ListPastes(context.Background(), limit, offset)
	if err != nil {
		return err
	}
	if jsonOut {
		return printJSON(out)
	}
	pastes, _ := out["pastes"].([]any)
	if pastes == nil {
		pastes, _ = out["items"].([]any)
	}
	for _, p := range pastes {
		m, _ := p.(map[string]any)
		if m == nil {
			continue
		}
		title := ""
		if t, ok := m["title"].(string); ok && t != "" {
			title = "  " + t
		}
		fmt.Printf("%v  %v  %v%s\n", m["id"], m["visibility"], m["created_at"], title)
	}
	return nil
}

func cmdPasteDelete(args []string) error {
	fs := flag.NewFlagSet("delete", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	var api, key string
	var yes bool
	fs.StringVar(&api, "api", "", "")
	fs.StringVar(&key, "key", "", "")
	fs.BoolVar(&yes, "yes", false, "confirm delete")
	fs.BoolVar(&yes, "y", false, "confirm delete")
	positionals, err := parseFlagsAllowInterspersed(fs, args)
	if err != nil {
		return err
	}
	if len(positionals) != 1 {
		return errors.New("usage: voidpaste delete <id> --yes")
	}
	if !yes {
		return errors.New("refusing to delete without --yes")
	}
	cfg, err := resolve(api, key)
	if err != nil {
		return err
	}
	if err := requireKey(cfg); err != nil {
		return err
	}
	if err := NewClient(cfg).DeletePaste(context.Background(), positionals[0]); err != nil {
		return err
	}
	fmt.Println("deleted")
	return nil
}

func cmdCollection(args []string) error {
	if len(args) == 0 {
		return errors.New("usage: voidpaste collection <list|create|get|delete|add|remove>")
	}
	switch args[0] {
	case "list", "ls":
		return cmdCollectionList(args[1:])
	case "create":
		return cmdCollectionCreate(args[1:])
	case "get":
		return cmdCollectionGet(args[1:])
	case "delete", "rm":
		return cmdCollectionDelete(args[1:])
	case "add":
		return cmdCollectionAdd(args[1:])
	case "remove":
		return cmdCollectionRemove(args[1:])
	default:
		return fmt.Errorf("unknown collection command %q", args[0])
	}
}

func cmdCollectionList(args []string) error {
	fs := flag.NewFlagSet("collection list", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	api, key, jsonOut, _, err := parseGlobal(fs, args)
	if err != nil {
		return err
	}
	cfg, err := resolve(api, key)
	if err != nil {
		return err
	}
	if err := requireKey(cfg); err != nil {
		return err
	}
	out, err := NewClient(cfg).ListCollections(context.Background())
	if err != nil {
		return err
	}
	if jsonOut {
		return printJSON(out)
	}
	cols, _ := out["collections"].([]any)
	for _, c := range cols {
		m, _ := c.(map[string]any)
		if m == nil {
			continue
		}
		fmt.Printf("%v  %v  %v\n", m["id"], m["visibility"], m["name"])
	}
	return nil
}

func cmdCollectionCreate(args []string) error {
	fs := flag.NewFlagSet("collection create", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	var api, key, name, desc, visibility string
	var jsonOut bool
	fs.StringVar(&api, "api", "", "")
	fs.StringVar(&key, "key", "", "")
	fs.StringVar(&name, "name", "", "collection name")
	fs.StringVar(&desc, "description", "", "description")
	fs.StringVar(&visibility, "visibility", "private", "public|unlisted|private")
	fs.BoolVar(&jsonOut, "json", false, "print JSON")
	if _, err := parseFlagsAllowInterspersed(fs, args); err != nil {
		return err
	}
	if name == "" {
		return errors.New("--name is required")
	}
	cfg, err := resolve(api, key)
	if err != nil {
		return err
	}
	if err := requireKey(cfg); err != nil {
		return err
	}
	out, err := NewClient(cfg).CreateCollection(context.Background(), name, desc, visibility)
	if err != nil {
		return err
	}
	if jsonOut {
		return printJSON(out)
	}
	col, _ := out["collection"].(map[string]any)
	if col == nil {
		return printJSON(out)
	}
	fmt.Printf("created %v  %v\n", col["id"], col["name"])
	return nil
}

func cmdCollectionGet(args []string) error {
	fs := flag.NewFlagSet("collection get", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	api, key, jsonOut, rest, err := parseGlobal(fs, args)
	if err != nil {
		return err
	}
	if len(rest) != 1 {
		return errors.New("usage: voidpaste collection get <id>")
	}
	cfg, err := resolve(api, key)
	if err != nil {
		return err
	}
	if err := requireKey(cfg); err != nil {
		return err
	}
	out, err := NewClient(cfg).GetCollection(context.Background(), rest[0])
	if err != nil {
		return err
	}
	if jsonOut {
		return printJSON(out)
	}
	col, _ := out["collection"].(map[string]any)
	if col != nil {
		fmt.Printf("id:     %v\n", col["id"])
		fmt.Printf("name:   %v\n", col["name"])
		fmt.Printf("vis:    %v\n", col["visibility"])
	}
	items, _ := out["items"].([]any)
	for _, it := range items {
		m, _ := it.(map[string]any)
		if m == nil {
			continue
		}
		fmt.Printf("  paste %v  pos %v\n", m["paste_id"], m["position"])
	}
	return nil
}

func cmdCollectionDelete(args []string) error {
	fs := flag.NewFlagSet("collection delete", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	var api, key string
	var yes bool
	fs.StringVar(&api, "api", "", "")
	fs.StringVar(&key, "key", "", "")
	fs.BoolVar(&yes, "yes", false, "confirm")
	fs.BoolVar(&yes, "y", false, "confirm")
	rest, err := parseFlagsAllowInterspersed(fs, args)
	if err != nil {
		return err
	}
	if len(rest) != 1 {
		return errors.New("usage: voidpaste collection delete <id> --yes")
	}
	if !yes {
		return errors.New("refusing to delete without --yes")
	}
	cfg, err := resolve(api, key)
	if err != nil {
		return err
	}
	if err := requireKey(cfg); err != nil {
		return err
	}
	if err := NewClient(cfg).DeleteCollection(context.Background(), rest[0]); err != nil {
		return err
	}
	fmt.Println("deleted")
	return nil
}

func cmdCollectionAdd(args []string) error {
	fs := flag.NewFlagSet("collection add", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	api, key, jsonOut, rest, err := parseGlobal(fs, args)
	if err != nil {
		return err
	}
	if len(rest) != 2 {
		return errors.New("usage: voidpaste collection add <collectionId> <pasteId>")
	}
	cfg, err := resolve(api, key)
	if err != nil {
		return err
	}
	if err := requireKey(cfg); err != nil {
		return err
	}
	out, err := NewClient(cfg).AddPasteToCollection(context.Background(), rest[0], rest[1])
	if err != nil {
		return err
	}
	if jsonOut {
		return printJSON(out)
	}
	fmt.Printf("added %v to %v\n", rest[1], rest[0])
	return nil
}

func cmdCollectionRemove(args []string) error {
	fs := flag.NewFlagSet("collection remove", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	api, key, _, rest, err := parseGlobal(fs, args)
	if err != nil {
		return err
	}
	if len(rest) != 2 {
		return errors.New("usage: voidpaste collection remove <collectionId> <pasteId>")
	}
	cfg, err := resolve(api, key)
	if err != nil {
		return err
	}
	if err := requireKey(cfg); err != nil {
		return err
	}
	if err := NewClient(cfg).RemovePasteFromCollection(context.Background(), rest[0], rest[1]); err != nil {
		return err
	}
	fmt.Println("removed")
	return nil
}

func cmdVersions(args []string) error {
	if len(args) == 0 {
		return errors.New("usage: voidpaste versions <list|get|restore>")
	}
	switch args[0] {
	case "list", "ls":
		return cmdVersionsList(args[1:])
	case "get":
		return cmdVersionsGet(args[1:])
	case "restore":
		return cmdVersionsRestore(args[1:])
	default:
		return fmt.Errorf("unknown versions command %q", args[0])
	}
}

func cmdVersionsList(args []string) error {
	fs := flag.NewFlagSet("versions list", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	api, key, jsonOut, rest, err := parseGlobal(fs, args)
	if err != nil {
		return err
	}
	if len(rest) != 1 {
		return errors.New("usage: voidpaste versions list <pasteId>")
	}
	cfg, err := resolve(api, key)
	if err != nil {
		return err
	}
	if err := requireKey(cfg); err != nil {
		return err
	}
	out, err := NewClient(cfg).ListVersions(context.Background(), rest[0])
	if err != nil {
		return err
	}
	if jsonOut {
		return printJSON(out)
	}
	versions, _ := out["versions"].([]any)
	for _, v := range versions {
		m, _ := v.(map[string]any)
		if m == nil {
			continue
		}
		fmt.Printf("v%v  %v bytes  %v\n", m["version"], m["size_bytes"], m["created_at"])
	}
	return nil
}

func cmdVersionsGet(args []string) error {
	fs := flag.NewFlagSet("versions get", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	api, key, jsonOut, rest, err := parseGlobal(fs, args)
	if err != nil {
		return err
	}
	if len(rest) != 2 {
		return errors.New("usage: voidpaste versions get <pasteId> <n>")
	}
	n, err := strconv.Atoi(rest[1])
	if err != nil {
		return fmt.Errorf("version number: %w", err)
	}
	cfg, err := resolve(api, key)
	if err != nil {
		return err
	}
	if err := requireKey(cfg); err != nil {
		return err
	}
	out, err := NewClient(cfg).GetVersion(context.Background(), rest[0], n)
	if err != nil {
		return err
	}
	if jsonOut {
		return printJSON(out)
	}
	return printJSON(out)
}

func cmdVersionsRestore(args []string) error {
	fs := flag.NewFlagSet("versions restore", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	api, key, jsonOut, rest, err := parseGlobal(fs, args)
	if err != nil {
		return err
	}
	if len(rest) != 2 {
		return errors.New("usage: voidpaste versions restore <pasteId> <n>")
	}
	n, err := strconv.Atoi(rest[1])
	if err != nil {
		return fmt.Errorf("version number: %w", err)
	}
	cfg, err := resolve(api, key)
	if err != nil {
		return err
	}
	if err := requireKey(cfg); err != nil {
		return err
	}
	out, err := NewClient(cfg).RestoreVersion(context.Background(), rest[0], n)
	if err != nil {
		return err
	}
	if jsonOut {
		return printJSON(out)
	}
	fmt.Printf("restored version %d of %s\n", n, rest[0])
	return nil
}

func printJSON(v any) error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}
