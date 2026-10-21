package mpk

import "encoding/xml"

type feed struct {
	XMLName xml.Name `xml:"rss"`
	Channel *channel `xml:"channel"`
}

type channel struct {
	Items []item `xml:"item"`
}

type item struct {
	GUID  string    `xml:"guid"`
	Link  string    `xml:"link"`
	Title string    `xml:"title"`
	Event *rawEvent `xml:"event"`
}

type rawEvent struct {
	ID           string `xml:"uniqueid"`
	Name         string `xml:"name"`
	City         string `xml:"city"`
	Location     string `xml:"location"`
	SpecialField string `xml:"specialfield"`
	Status       string `xml:"status"`
	Start        string `xml:"startdate"`
	End          string `xml:"enddate"`
	Registration struct {
		Link string `xml:"link"`
	} `xml:"ilmolink"`
}
