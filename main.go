package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"strings"
	"time"

  "golang.org/x/text/language"
	"golang.org/x/text/message"
)

const (
	fullURL    = `https://www.tesla.com/inventory/api/v4/inventory-results?query=%7B%22query%22%3A%7B%22model%22%3A%22m3%22%2C%22condition%22%3A%22used%22%2C%22options%22%3A%7B%22TRIM%22%3A%5B%22LRAWD%22%2C%22LRRWD%22%5D%2C%22Year%22%3A%5B%222021%22%2C%222022%22%2C%222023%22%2C%222024%22%5D%7D%2C%22arrangeby%22%3A%22Price%22%2C%22order%22%3A%22asc%22%2C%22market%22%3A%22DE%22%2C%22language%22%3A%22de%22%2C%22super_region%22%3A%22north%20america%22%2C%22lng%22%3A11.0262%2C%22lat%22%3A49.3257%2C%22zip%22%3A%2291126%22%2C%22range%22%3A0%2C%22region%22%3A%22BY%22%7D%2C%22offset%22%3A0%2C%22count%22%3A24%2C%22outsideOffset%22%3A0%2C%22outsideSearch%22%3Afalse%2C%22isFalconDeliverySelectionEnabled%22%3Afalse%2C%22version%22%3Anull%7D`
	ntfyURL    = "https://ntfy.sh/tesla-alerts-23d47c8d601fc648fe171a2ddb60b0da"
	priceLimit = 28000
	seenFile   = "seen.json"
)

type TeslaResponse struct {
	Results []TeslaCar `json:"results"`
}

type Option struct {
	Code  string `json:"code"`
	Group string `json:"group"`
	Value string `json:"value"`
	Name  string `json:"name"`
}

type TeslaCar struct {
	VIN              string   `json:"VIN"`
	Price            float64  `json:"Price"`
	Odometer         int      `json:"Odometer"`
	Paint            []string `json:"PAINT"`
	Interior         []string `json:"INTERIOR"`
	Trim             []string `json:"TRIM"`
	Model            string   `json:"Model"`
	City             string   `json:"City"`
	Year             int      `json:"Year"`
	OptionCodeData   []Option `json:"OptionCodeData"`
	ADLOpts          []string `json:"ADL_OPTS"`
}

var seenCars = make(map[string]float64)

func main() {
	loadSeen()
	for {
		checkInventory()
		saveSeen()
		time.Sleep(10 * time.Minute)
	}
}

func checkInventory() {
	client := &http.Client{}

	req, err := http.NewRequest("GET", fullURL, nil)
	if err != nil {
		fmt.Println("Error creating request:", err)
		return
	}

	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64)")
	req.Header.Set("Accept", "application/json, text/plain, */*")
	req.Header.Set("Accept-Language", "de-DE,de;q=0.9,en;q=0.8")
	req.Header.Set("Referer", "https://www.tesla.com/")
	req.Header.Set("Origin", "https://www.tesla.com")

	resp, err := client.Do(req)
	if err != nil {
		fmt.Println("Request failed:", err)
		return
	}
	defer resp.Body.Close()

	body, _ := ioutil.ReadAll(resp.Body)

	var teslaResp TeslaResponse
	err = json.Unmarshal(body, &teslaResp)
	if err != nil {
		fmt.Println("JSON parse failed:", err)
		return
	}

	for _, car := range teslaResp.Results {
		prevPrice, seen := seenCars[car.VIN]

		if car.Price < priceLimit && (!seen || prevPrice != car.Price) {
			notify(car)
			seenCars[car.VIN] = car.Price
		}
	}
}

func notify(car TeslaCar) {
	// Get range info
	rangeInfo := ""
	for _, opt := range car.OptionCodeData {
		if opt.Group == "SPECS_RANGE" && opt.Value != "" {
			rangeInfo = opt.Value + " km"
			break
		}
	}

	// Towing & Acceleration Boost
	towing := "❌"
	boost := ""

	for _, a := range car.ADLOpts {
		if strings.Contains(a, "TOWING") {
			towing = "✔️"
		}
		if strings.Contains(a, "ACCELERATION_BOOST") {
			boost = "🚀 Acceleration Boost"
		}
	}

	
  p := message.NewPrinter(language.German)

  msg := fmt.Sprintf("🚗 %s (%d) in %s\n💶 %s€ • %s km\n🎨 %s\n🪑 %s\n🔋 %s\n🧲 Towing: %s %s",
	  car.Model,
	  car.Year,
	  car.City,
	  p.Sprintf("%.0f", car.Price),
	  p.Sprintf("%d", car.Odometer),
	  strings.Join(car.Paint, ", "),
	  strings.Join(car.Interior, ", "),
	  rangeInfo,
	  towing,
	  boost,
  )


	  http.Post(ntfyURL, "text/plain", strings.NewReader(msg))
	  fmt.Println("🔔 New car posted to ntfy:", msg)
  }

func loadSeen() {
	data, err := os.ReadFile(seenFile)
	if err == nil {
		json.Unmarshal(data, &seenCars)
	}
}

func saveSeen() {
	data, _ := json.MarshalIndent(seenCars, "", "  ")
	_ = os.WriteFile(seenFile, data, 0644)
}

