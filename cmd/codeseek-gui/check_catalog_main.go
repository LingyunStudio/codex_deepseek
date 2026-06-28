// +build ignore

package main

import (
	"encoding/json"
	"fmt"
	"os"
	"codeseek/internal/config"
	"codeseek/internal/extension/codex"
	"codeseek/internal/service/app"
)

func main() {
	cfg, _ := config.LoadFromFileWithOptions("../../config.example.yml", config.LoadOptions{
		ExtensionSpecs: app.BuiltinExtensions().ConfigSpecs(),
	})
	pc := config.ProviderFromGlobalConfig(&cfg)
	pl := config.PluginFromGlobalConfig(&cfg)
	models := codex.BuildModelInfosFromConfig(pc, pl)
	data, _ := json.MarshalIndent(map[string]any{"models": models}, "", "  ")
	os.WriteFile("catalog_test.json", data, 0644)
	fmt.Println("Generated", len(models), "models")
	var wrapper struct{ Models []map[string]any }
	json.Unmarshal(data, &wrapper)
	if len(wrapper.Models) > 0 {
		fmt.Println("Fields in first model:")
		for k := range wrapper.Models[0] {
			fmt.Println(" -", k)
		}
	}
}
