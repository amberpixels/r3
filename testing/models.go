package depotesting

import "time"

// City represents a geographical city.
type City struct {
	ID          int64 `gorm:"primaryKey"`
	Name        string
	CountryName string
	Popularity  int

	// Association: one-to-many with CityTranslation
	Translations []CityTranslation `gorm:"foreignKey:CityID"`
}

// CityTranslation stores the translated name for a City.
type CityTranslation struct {
	ID     int64 `gorm:"primaryKey"`
	Name   string
	CityID int64  // Foreign key to City.ID
	Locale string // e.g., "en", "es", "de"
}

// Location represents a location that belongs to a city.
type Location struct {
	ID         int64 `gorm:"primaryKey"`
	Name       string
	Slug       string
	CityID     int64
	Popularity int
	Visible    bool

	City         *City                 `gorm:"foreignKey:CityID"`
	Translations []LocationTranslation `gorm:"foreignKey:LocationID"`
}

// LocationTranslation stores the translated name and slug for a Location.
type LocationTranslation struct {
	ID         int64 `gorm:"primaryKey"`
	Name       string
	Slug       string
	LocationID int64  // Foreign key to Location.ID
	Locale     string // e.g., "en", "es", "de"
}

// Event represents an event associated with a location.
type Event struct {
	ID         int64 `gorm:"primaryKey"`
	HappenedAt time.Time
	Name       string
	Weight     int
	LocationID int64 // relates to Location.ID
	Active     bool

	Location *Location `gorm:"foreignKey:LocationID"`

	Artists      []Artist           `gorm:"many2many:artist_to_events"`
	Translations []EventTranslation `gorm:"foreignKey:EventID"`
}

// EventTranslation stores the translated name for an Event.
type EventTranslation struct {
	ID      int64 `gorm:"primaryKey"`
	Name    string
	EventID int64  // Foreign key to Event.ID
	Locale  string // e.g., "en", "es", "de"
}

// Artist represents a person who performs at events.
type Artist struct {
	ID   int64 `gorm:"primaryKey"`
	Name string
	// Add more fields as needed (e.g., Bio, Genre, etc.)
	Translations []ArtistTranslation `gorm:"foreignKey:ArtistID"`
}

// ArtistTranslation stores the translated name for an Artist.
type ArtistTranslation struct {
	ID       int64 `gorm:"primaryKey"`
	Name     string
	ArtistID int64  // Foreign key to Artist.ID
	Locale   string // e.g., "en", "es", "de"
}

// ArtistToEvent represents the many-to-many relationship between artists and events.
type ArtistToEvent struct {
	ArtistID int64 `gorm:"primaryKey"`
	EventID  int   `gorm:"primaryKey"`
}
