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
}
