package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"game-engine/internal/assets"
	"game-engine/internal/commands"
	"game-engine/internal/download"
	"game-engine/internal/fonts"
	"game-engine/internal/googlefonts"
	"game-engine/internal/mapgen"
	"game-engine/internal/scene"
)

// Download destinations, relative to the working directory.
const (
	textureDownloadDir = "assets/textures/downloaded"
	skyboxDownloadDir  = "assets/skybox/downloaded"
	fontDownloadDir    = "assets/fonts/downloaded"
)

// registerAssetCommands registers texture, skybox, font, and terrain commands.
func (a *App) registerAssetCommands() {
	scn := a.scene

	dl := commands.Command{
		Name: "download", Usage: "image <url>",
		Help: "Download an image in the background and apply it as the selected object's texture. Select an object first.",
	}
	dl.Run = a.withSelected(func(o *scene.Object, args []string) error {
		if len(args) != 2 || args[0] != "image" {
			return dl.UsageError()
		}
		id, url := o.ID, args[1]
		a.log.Log("Downloading image…")
		background(a, func() (string, error) {
			return download.Download(url, textureDownloadDir)
		}, func(path string, err error) {
			if err != nil {
				a.log.Log(err.Error())
				return
			}
			o, ok := scn.Object(id)
			if !ok {
				a.log.Log("Image saved to " + path + ", but its object was deleted.")
				return
			}
			o.Texture = path
			a.log.Log("Texture applied: " + path)
		})
		return nil
	})
	a.commands.Register(dl)

	a.commands.Register(commands.Command{
		Name: "texture", Usage: "<path>",
		Help: "Apply an image file as the selected object's texture (e.g. assets/textures/downloaded/foo.png). Select an object first.",
		Run: a.withSelected(func(o *scene.Object, args []string) error {
			if len(args) != 1 {
				return errors.New("usage: cmd texture <path>")
			}
			o.Texture = args[0]
			return nil
		}),
	})

	sky := commands.Command{
		Name: "skybox", Usage: "<url>",
		Help: "Download an image in the background and use it as the skybox (2:1 panorama or cubemap layout).",
	}
	sky.Run = func(args []string) error {
		if len(args) != 1 {
			return sky.UsageError()
		}
		url := args[0]
		a.log.Log("Downloading skybox…")
		background(a, func() (string, error) {
			return download.Download(url, skyboxDownloadDir)
		}, func(path string, err error) {
			if err == nil {
				err = scn.SetSkyboxPath(path)
			}
			if err != nil {
				a.log.Log("skybox: " + err.Error())
				return
			}
			a.log.Log("Skybox set: " + path)
		})
		return nil
	}
	a.commands.Register(sky)

	a.commands.Register(commands.Command{
		Name: "font", Usage: "[<name>]",
		Help: "Set the UI font by family name (e.g. Inter, Open Sans) or path under assets/fonts/. " +
			"Fonts not installed locally are downloaded from Google Fonts. No argument shows the current font.",
		Run: func(args []string) error {
			if len(args) == 0 {
				a.log.Log("Current font: " + a.font)
				return nil
			}
			return a.setFont(fonts.StripAssetsFontsPrefix(strings.Join(args, " ")))
		},
	})

	hm := commands.Command{
		Name: "heightmap", Usage: "[--w <tiles>] [--d <tiles>] [--tile <size>] [--h <height>] [--seed <n>]",
		Help: "Generate a static procedural terrain mesh centered on the origin, replacing any existing terrain.",
	}
	hm.Run = func(args []string) error {
		opts := mapgen.DefaultHeightMapOptions()
		var tile, height float64
		fs := commands.NewFlagSet("heightmap")
		fs.IntVar(&opts.Width, "w", opts.Width, "width in tiles")
		fs.IntVar(&opts.Depth, "d", opts.Depth, "depth in tiles")
		fs.Float64Var(&tile, "tile", float64(opts.TileSize), "tile size on X/Z")
		fs.Float64Var(&height, "h", float64(opts.HeightScale), "max height")
		fs.Int64Var(&opts.Seed, "seed", 0, "random seed (0 = random)")
		if err := fs.Parse(args); err != nil {
			return fmt.Errorf("%w; %w", err, hm.UsageError())
		}
		opts.TileSize, opts.HeightScale = float32(tile), float32(height)
		mesh, size, err := mapgen.GenerateTerrain(opts)
		if err != nil {
			return err
		}
		scn.SetTerrain(mesh, size)
		a.log.Log(fmt.Sprintf("Heightmap generated (%.0f×%.0f, max height %.1f).", size[0], size[2], size[1]))
		return nil
	}
	a.commands.Register(hm)

	repeat := commands.Command{Name: "terrain_repeat", Usage: "<u> <v>", Help: "Tile the terrain texture u×v times (1 1 stretches it once)."}
	repeat.Run = func(args []string) error {
		if len(args) != 2 {
			return repeat.UsageError()
		}
		uv, err := parseFloats("repeat", args)
		if err != nil {
			return err
		}
		if uv[0] <= 0 || uv[1] <= 0 {
			return errors.New("terrain_repeat: u and v must be > 0")
		}
		scn.SetTerrainTextureRepeat(uv[0], uv[1])
		a.log.Log(fmt.Sprintf("Terrain texture repeat set to %.2fx, %.2fy.", uv[0], uv[1]))
		return nil
	}
	a.commands.Register(repeat)
}

// setFont loads a font by path under assets/fonts/ or by (fuzzy) family name, downloading it from
// Google Fonts in the background when it isn't installed.
func (a *App) setFont(name string) error {
	if path, ok := assets.Find("assets/fonts/" + name); ok {
		if err := a.loadFont(name, path); err == nil {
			a.fontChanged()
			return nil
		}
	}
	for _, search := range fonts.SearchCandidates(name) {
		rel, path, err := fonts.FindFont(search)
		if err != nil {
			continue
		}
		if err := a.loadFont(rel, path); err == nil {
			a.fontChanged()
			return nil
		}
	}

	a.log.Log("Downloading font from Google Fonts…")
	type result struct{ rel, path string }
	background(a, func() (result, error) {
		url, err := googlefonts.FetchDownloadURLByFamily(name)
		if err != nil {
			return result{}, err
		}
		folder := googlefonts.NormalizeFamily(name)[0]
		dir := filepath.Join(fontDownloadDir, folder)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return result{}, err
		}
		path, err := download.Download(url, dir)
		if err != nil {
			return result{}, err
		}
		return result{rel: "downloaded/" + folder + "/" + filepath.Base(path), path: path}, nil
	}, func(r result, err error) {
		if err == nil {
			err = a.loadFont(r.rel, r.path)
		}
		if err != nil {
			a.log.Log("font: " + err.Error())
			return
		}
		a.fontChanged()
	})
	return nil
}

func (a *App) fontChanged() {
	a.savePrefs()
	a.log.Log("Font set: " + a.font)
}
