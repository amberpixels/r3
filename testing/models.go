package depotesting

import "time"

// City represents a geographical city.
type City struct {
	ID          int64 `gorm:"primaryKey"`
	Name        string
	CountryName string
	Popularity  int
}

// Location represents a location that belongs to a city.
type Location struct {
	ID         int64 `gorm:"primaryKey"`
	Name       string
	Slug       string
	CityID     int64
	Popularity int
	Visible    bool

	City *City `gorm:"foreignKey:CityID"`
}

// Event represents an event associated with a location.
type Event struct {
	ID         int64 `gorm:"primaryKey"`
	HappenedAt time.Time
	Weight     int
	VenueID    int64 // relates to Location.ID
	Active     bool

	Location *Location `gorm:"foreignKey:VenueID"`

	Artists []Artist `gorm:"many2many:artist_to_events"`
}

// Artist represents a person who performs at events.
type Artist struct {
	ID   int64 `gorm:"primaryKey"`
	Name string
	// Add more fields as needed (e.g., Bio, Genre, etc.)
}

// ArtistToEvent represents the many-to-many relationship between artists and events.
type ArtistToEvent struct {
	ArtistID int64 `gorm:"primaryKey"`
	EventID  int   `gorm:"primaryKey"`
}
