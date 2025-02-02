package layout

import (
	"log"
	"path/filepath"
	"photofield/internal/image"
	"photofield/internal/render"
	"strings"
	"time"
)

type Type string

const (
	Album    Type = "ALBUM"
	Timeline Type = "TIMELINE"
	Square   Type = "SQUARE"
	Wall     Type = "WALL"
)

type Layout struct {
	Type         Type `json:"type"`
	SceneWidth   float64
	ImageHeight  float64
	ImageSpacing float64
	LineSpacing  float64
}

type Section struct {
	infos    []image.SourcedInfo
	Inverted bool
}

type SectionPhoto struct {
	Index int
	Photo *render.Photo
	Size  image.Size
}

type Photo struct {
	Index int
	Photo render.Photo
	Info  image.Info
}

type PhotoRegionSource struct {
	Source *image.Source
}

type RegionThumbnail struct {
	Name   string `json:"name"`
	Width  int    `json:"width"`
	Height int    `json:"height"`
}

type PhotoRegionData struct {
	Id         int               `json:"id"`
	Path       string            `json:"path"`
	Filename   string            `json:"filename"`
	Extension  string            `json:"extension"`
	Video      bool              `json:"video"`
	Width      int               `json:"width"`
	Height     int               `json:"height"`
	CreatedAt  string            `json:"created_at"`
	Thumbnails []RegionThumbnail `json:"thumbnails"`
	// SmallestThumbnail     string   `json:"smallest_thumbnail"`
}

func (regionSource PhotoRegionSource) getRegionFromPhoto(id int, photo *render.Photo, scene *render.Scene, regionConfig render.RegionConfig) render.Region {

	source := regionSource.Source

	originalPath := photo.GetPath(source)
	info := source.GetInfo(photo.Id)
	originalSize := image.Size{
		X: info.Width,
		Y: info.Height,
	}
	isVideo := source.IsSupportedVideo(originalPath)

	var thumbnailTemplates []image.Thumbnail
	if isVideo {
		thumbnailTemplates = source.Videos.Thumbnails
	} else {
		thumbnailTemplates = source.Images.Thumbnails
	}

	var thumbnails []RegionThumbnail
	for i := range thumbnailTemplates {
		thumbnail := &thumbnailTemplates[i]
		thumbnailPath := thumbnail.GetPath(originalPath)
		if source.Exists(thumbnailPath) {
			thumbnailSize := thumbnail.Fit(originalSize)
			thumbnails = append(thumbnails, RegionThumbnail{
				Name:   thumbnail.Name,
				Width:  thumbnailSize.X,
				Height: thumbnailSize.Y,
			})
		}
	}

	return render.Region{
		Id:     id,
		Bounds: photo.Sprite.Rect,
		Data: PhotoRegionData{
			Id:         int(photo.Id),
			Path:       originalPath,
			Filename:   filepath.Base(originalPath),
			Extension:  strings.ToLower(filepath.Ext(originalPath)),
			Video:      isVideo,
			Width:      info.Width,
			Height:     info.Height,
			CreatedAt:  info.DateTime.Format(time.RFC3339),
			Thumbnails: thumbnails,
		},
	}
}

func (regionSource PhotoRegionSource) GetRegionsFromBounds(rect render.Rect, scene *render.Scene, regionConfig render.RegionConfig) []render.Region {
	regions := make([]render.Region, 0)
	photos := scene.GetVisiblePhotos(rect, regionConfig.Limit)
	for photo := range photos {
		regions = append(regions, regionSource.getRegionFromPhoto(
			photo.Index,
			photo.Photo,
			scene, regionConfig,
		))
	}
	return regions
}

func (regionSource PhotoRegionSource) GetRegionById(id int, scene *render.Scene, regionConfig render.RegionConfig) render.Region {
	if id < 0 || id >= len(scene.Photos)-1 {
		return render.Region{Id: -1}
	}
	photo := scene.Photos[id]
	return regionSource.getRegionFromPhoto(id, &photo, scene, regionConfig)
}

func layoutFitRow(row []SectionPhoto, bounds render.Rect, imageSpacing float64) float64 {
	count := len(row)
	if count == 0 {
		return 1.
	}
	firstPhoto := row[0]
	firstRect := firstPhoto.Photo.Sprite.Rect
	lastPhoto := row[count-1]
	lastRect := lastPhoto.Photo.Sprite.Rect
	totalSpacing := float64(count-1) * imageSpacing

	rowWidth := lastRect.X + lastRect.W
	scale := (bounds.W - totalSpacing) / (rowWidth - totalSpacing)
	x := firstRect.X
	for i := range row {
		photo := row[i]
		rect := photo.Photo.Sprite.Rect
		photo.Photo.Sprite.Rect = render.Rect{
			X: x,
			Y: rect.Y,
			W: rect.W * scale,
			H: rect.H * scale,
		}
		x += photo.Photo.Sprite.Rect.W + imageSpacing
	}

	// fmt.Printf("fit row width %5.2f / %5.2f -> %5.2f  scale %.2f\n", rowWidth, bounds.W, lastPhoto.Photo.Original.Sprite.Rect.X+lastPhoto.Photo.Original.Sprite.Rect.W, scale)

	x -= imageSpacing
	return scale
}

func addSectionPhotos(section *Section, scene *render.Scene, source *image.Source) <-chan SectionPhoto {
	photos := make(chan SectionPhoto, 10000)
	go func() {
		startIndex := len(scene.Photos)
		for _, info := range section.infos {
			scene.Photos = append(scene.Photos, render.Photo{
				Id:     info.Id,
				Sprite: render.Sprite{},
			})
		}
		for index, info := range section.infos {
			sceneIndex := startIndex + index
			photo := &scene.Photos[sceneIndex]
			photos <- SectionPhoto{
				Index: sceneIndex,
				Photo: photo,
				Size: image.Size{
					X: info.Width,
					Y: info.Height,
				},
			}
		}
		close(photos)
	}()
	return photos
}

func layoutSectionPhotos(photos <-chan SectionPhoto, bounds render.Rect, config Layout, scene *render.Scene, source *image.Source) render.Rect {
	x := 0.
	y := 0.
	lastLogTime := time.Now()
	i := 0

	row := make([]SectionPhoto, 0)

	for photo := range photos {

		// log.Println("layout", photo.Index)

		aspectRatio := float64(photo.Size.X) / float64(photo.Size.Y)
		imageWidth := float64(config.ImageHeight) * aspectRatio

		if x+imageWidth > bounds.W {
			scale := layoutFitRow(row, bounds, config.ImageSpacing)
			row = nil
			x = 0
			y += config.ImageHeight*scale + config.LineSpacing
		}

		// fmt.Printf("%4.0f %4.0f %4.0f %4.0f %4.0f %4.0f %4.0f\n", bounds.X, bounds.Y, x, y, imageHeight, photo.Size.Width, photo.Size.Height)

		photo.Photo.Sprite.PlaceFitHeight(
			bounds.X+x,
			bounds.Y+y,
			config.ImageHeight,
			float64(photo.Size.X),
			float64(photo.Size.Y),
		)

		row = append(row, photo)

		// photoRect := photo.Photo.Original.Sprite.GetBounds()
		// scene.Regions = append(scene.Regions, Region{
		// 	Id: len(scene.Regions),
		// 	Bounds: Bounds{
		// 		X: photoRect.X,
		// 		Y: photoRect.Y,
		// 		W: photoRect.W,
		// 		H: photoRect.H,
		// 	},
		// })

		// fmt.Printf("%d %f %f %f\n", i, x, imageWidth, bounds.W)

		x += imageWidth + config.ImageSpacing

		now := time.Now()
		if now.Sub(lastLogTime) > 1*time.Second {
			lastLogTime = now
			log.Printf("layout section %d\n", photo.Index)
		}
		i++
	}
	x = 0
	y += config.ImageHeight + config.LineSpacing
	return render.Rect{
		X: bounds.X,
		Y: bounds.Y,
		W: bounds.W,
		H: y,
	}
}

func SameDay(a, b time.Time) bool {
	y1, m1, d1 := a.Date()
	y2, m2, d2 := b.Date()
	return y1 == y2 && m1 == m2 && d1 == d2
}
