package photofield

import (
	"log"
	"time"

	. "photofield/internal"
	. "photofield/internal/display"
	storage "photofield/internal/storage"

	"github.com/hako/durafmt"
	"github.com/tdewolff/canvas"
)

type Event struct {
	StartTime time.Time
	EndTime   time.Time
	Section   Section
}

func LayoutTimelineEvent(config LayoutConfig, rect Rect, event *Event, scene *Scene, source *storage.ImageSource) Rect {

	imageHeight := config.ImageHeight
	imageSpacing := 3.
	lineSpacing := 3.

	// log.Println("layout event", len(event.Section.photos), rect.X, rect.Y)

	textHeight := 30.
	textBounds := Rect{
		X: rect.X,
		Y: rect.Y,
		W: rect.W,
		H: textHeight,
	}

	startTimeFormat := "Mon, Jan 2, 15:04"
	if event.StartTime.Year() != time.Now().Year() {
		startTimeFormat = "Mon, Jan 2, 2006, 15:04"
	}

	headerText := event.StartTime.Format(startTimeFormat)

	if SameDay(event.StartTime, event.EndTime) {
		// endTimeFormat = "15:04"
		duration := event.EndTime.Sub(event.StartTime)
		if duration >= 1*time.Minute {
			dur := durafmt.Parse(duration)
			headerText += "   " + dur.LimitFirstN(1).String()
		}
	} else {
		headerText += " - " + event.EndTime.Format(startTimeFormat)
	}

	font := config.FontFamily.Face(40, canvas.Black, canvas.FontRegular, canvas.FontNormal)

	scene.Texts = append(scene.Texts,
		NewTextFromRect(
			textBounds,
			&font,
			headerText,
		),
	)
	rect.Y += textHeight + 15

	photos := make(chan SectionPhoto, 1)
	boundsOut := make(chan Rect)
	go layoutSectionPhotos(photos, rect, boundsOut, imageHeight, imageSpacing, lineSpacing, scene, source)
	go getSectionPhotos(&event.Section, photos, source)
	newBounds := <-boundsOut

	rect.Y = newBounds.Y + newBounds.H
	rect.Y += 10
	return rect
}

func LayoutTimelineEvents(config LayoutConfig, scene *Scene, source *storage.ImageSource) {

	// log.Println("layout")

	// log.Println("layout load info")
	layoutPhotos := getLayoutPhotos(scene.Photos, source)
	sortOldestToNewest(layoutPhotos)

	count := len(layoutPhotos)
	if config.Limit > 0 && config.Limit < count {
		count = config.Limit
	}

	scene.Photos = scene.Photos[0:count]
	layoutPhotos = layoutPhotos[0:count]

	sceneMargin := 10.

	scene.Bounds.W = config.SceneWidth

	event := Event{}
	eventCount := 0
	var lastPhotoTime time.Time

	rect := Rect{
		X: sceneMargin,
		Y: sceneMargin,
		W: scene.Bounds.W - sceneMargin*2,
		H: 0,
	}

	if config.FontFamily == nil {
		config.FontFamily = canvas.NewFontFamily("Roboto")
		err := config.FontFamily.LoadFontFile("fonts/Roboto/Roboto-Regular.ttf", canvas.FontRegular)
		if err != nil {
			panic(err)
		}
		err = config.FontFamily.LoadFontFile("fonts/Roboto/Roboto-Bold.ttf", canvas.FontBold)
		if err != nil {
			panic(err)
		}
	}
	if config.HeaderFont == nil {
		face := config.FontFamily.Face(80.0, canvas.Gray, canvas.FontRegular, canvas.FontNormal)
		config.HeaderFont = &face
	}

	scene.Solids = make([]Solid, 0)
	scene.Texts = make([]Text, 0)

	// log.Println("layout placing")
	layoutPlaced := ElapsedWithCount("layout placing", count)
	lastLogTime := time.Now()
	for i := range scene.Photos {
		if i >= count {
			break
		}
		LayoutPhoto := &layoutPhotos[i]
		scene.Photos[i] = LayoutPhoto.Photo
		photo := &scene.Photos[i]
		info := LayoutPhoto.Info

		photoTime := info.DateTime
		elapsed := lastPhotoTime.Sub(photoTime)
		if elapsed > 10*time.Minute {
			event.StartTime = lastPhotoTime
			rect = LayoutTimelineEvent(config, rect, &event, scene, source)
			eventCount++
			event = Event{}
			event.EndTime = photoTime
		}
		lastPhotoTime = photoTime

		event.Section.photos = append(event.Section.photos, photo)

		now := time.Now()
		if now.Sub(lastLogTime) > 1*time.Second {
			lastLogTime = now
			log.Printf("layout %d / %d\n", i, count)
		}
	}
	layoutPlaced()

	if len(event.Section.photos) > 0 {
		event.StartTime = lastPhotoTime
		rect = LayoutTimelineEvent(config, rect, &event, scene, source)
		eventCount++
	}

	log.Printf("layout events %d\n", eventCount)

	scene.Bounds.H = rect.Y + sceneMargin
	scene.RegionSource = PhotoRegionSource{
		imageSource: source,
	}

}
