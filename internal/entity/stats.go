package entity

import "time"

type StatsRequest struct {
	From string `json:"from"`
	To   string `json:"to"`
}

type Point struct {
	TS time.Time `json:"ts"`
	V  int64     `json:"v"`
}

type StatsResponse struct {
	Stats []Point `json:"stats"`
}
