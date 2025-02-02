package scene

import (
	"log"
	"sync"
	"time"
	"unsafe"

	"github.com/dgraph-io/ristretto"
	gonanoid "github.com/matoous/go-nanoid/v2"

	"photofield/internal/collection"
	"photofield/internal/image"
	"photofield/internal/layout"
	"photofield/internal/metrics"
	"photofield/internal/render"
)

type SceneSource struct {
	DefaultScene render.Scene

	sceneCache *ristretto.Cache
	scenes     sync.Map
}

type loadingScene struct {
	scene  *render.Scene
	loaded chan struct{}
}

type storedScene struct {
	scene  *render.Scene
	config SceneConfig
}

type SceneConfig struct {
	Render     render.Render
	Collection collection.Collection
	Layout     layout.Layout
	Scene      render.Scene
}

func NewSceneSource() *SceneSource {
	var err error
	source := SceneSource{}
	source.sceneCache, err = ristretto.NewCache(&ristretto.Config{
		NumCounters: 1e4,     // number of keys to track frequency of, 10x max expected key count
		MaxCost:     1 << 27, // maximum size/cost of cache (128MiB)
		BufferItems: 64,      // number of keys per Get buffer.
		Metrics:     true,
	})
	if err != nil {
		panic(err)
	}
	metrics.AddRistretto("scene_cache", source.sceneCache)
	return &source
}

func getSceneCost(scene *render.Scene) int64 {
	structCost := (int64)(unsafe.Sizeof(*scene))
	photosCost := (int64)(len(scene.Photos)) * (int64)(unsafe.Sizeof(scene.Photos[0]))
	solidsCost := (int64)(len(scene.Solids)) * (int64)(unsafe.Sizeof(scene.Solids[0]))
	textsCost := (int64)(len(scene.Texts)) * ((int64)(unsafe.Sizeof(scene.Solids[0])) + (int64)(100))
	return structCost + photosCost + solidsCost + textsCost
}

func (source *SceneSource) loadScene(config SceneConfig, imageSource *image.Source) render.Scene {
	log.Printf("scene loading %v", config.Collection.Id)

	finished := metrics.Elapsed("scene load " + config.Collection.Id)

	scene := source.DefaultScene

	switch config.Layout.Type {
	case layout.Timeline:
		layout.LayoutTimeline(config.Layout, config.Collection, &scene, imageSource)

	case layout.Album:
		layout.LayoutAlbum(config.Layout, config.Collection, &scene, imageSource)

	case layout.Square:
		layout.LayoutSquare(&scene, imageSource)

	case layout.Wall:
		layout.LayoutWall(config.Layout, config.Collection, &scene, imageSource)

	default:
		layout.LayoutAlbum(config.Layout, config.Collection, &scene, imageSource)
	}

	if scene.RegionSource == nil {
		scene.RegionSource = &layout.PhotoRegionSource{
			Source: imageSource,
		}
	}

	scene.FileCount = len(scene.Photos)
	scene.CreatedAt = time.Now()
	finished()

	log.Printf("photos %d, scene %.0f x %.0f\n", len(scene.Photos), scene.Bounds.W, scene.Bounds.H)
	return scene
}

func (source *SceneSource) GetSceneById(id string, imageSource *image.Source) *render.Scene {
	value, found := source.sceneCache.Get(id)
	if found {
		return value.(*render.Scene)
	}

	stored, loaded := source.scenes.Load(id)
	if loaded {
		scene := stored.(storedScene).scene
		source.sceneCache.Set(id, scene, getSceneCost(scene))
		return scene
	}
	return nil
}

func sceneConfigEqual(a SceneConfig, b SceneConfig) bool {
	if a.Collection.Limit != b.Collection.Limit {
		return false
	}
	if a.Collection.IndexLimit != b.Collection.IndexLimit {
		return false
	}
	for _, dirA := range a.Collection.Dirs {
		found := false
		for _, dirB := range b.Collection.Dirs {
			if dirA == dirB {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	if a.Layout.SceneWidth != 0 &&
		b.Layout.SceneWidth != 0 &&
		a.Layout.SceneWidth != b.Layout.SceneWidth {
		return false
	}

	if a.Layout.ImageHeight != 0 &&
		b.Layout.ImageHeight != 0 &&
		a.Layout.ImageHeight != b.Layout.ImageHeight {
		return false
	}

	return a.Layout.Type != "" &&
		b.Layout.Type != "" &&
		a.Layout.Type == b.Layout.Type
}

func (source *SceneSource) GetScenesWithConfig(config SceneConfig) []*render.Scene {
	scenes := make([]*render.Scene, 0)
	source.scenes.Range(func(_, value interface{}) bool {
		stored := value.(storedScene)
		if sceneConfigEqual(stored.config, config) {
			scenes = append(scenes, stored.scene)
		}
		return true
	})
	return scenes
}

func (source *SceneSource) Add(config SceneConfig, imageSource *image.Source) *render.Scene {

	id := config.Scene.Id
	if id == "" {
		var err error
		id, err = gonanoid.Generate("6789BCDFGHJKLMNPQRTWbcdfghjkmnpqrtwz", 10)
		if err != nil {
			panic(err)
		}
	}

	scene := source.loadScene(config, imageSource)
	scene.Id = id

	source.scenes.Store(scene.Id, storedScene{
		scene:  &scene,
		config: config,
	})
	return &scene
}
