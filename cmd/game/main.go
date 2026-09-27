// Command game runs the LLM-driven game engine.
package main

import (
	"net/http"
	_ "net/http/pprof"
	"os"

	"game-engine/internal/assets"
	"game-engine/internal/env"
)

func main() {
	for _, p := range assets.Candidates(".env") {
		_ = env.Load(p)
	}
	if os.Getenv("DEBUG_PPROF") == "1" {
		go func() { _ = http.ListenAndServe("localhost:6060", nil) }()
	}
	NewApp().Run()
}
