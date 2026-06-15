package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/NimbleMarkets/ntcharts/v2/canvas"
	"github.com/NimbleMarkets/ntcharts/v2/linechart/timeserieslinechart"
)

type Sample struct {
	Timestamp time.Time `json:"ts"`
	Type      string    `json:"type"`
	LatencyMS int       `json:"latency_ms"`
	Internet  bool      `json:"internet"`
}

type Event struct {
	Timestamp time.Time `json:"ts"`
	Type      string    `json:"type"`
	Event     string    `json:"event"`
	Message   string    `json:"message"`
}

func logFilePath() string {
	var base string
	switch runtime.GOOS {
	case "windows":
		base = os.Getenv("APPDATA")
	case "darwin":
		home, _ := os.UserHomeDir()
		base = filepath.Join(home, "Library", "Application Support")
	default:
		if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
			base = xdg
		} else {
			home, _ := os.UserHomeDir()
			base = filepath.Join(home, ".config")
		}
	}
	return filepath.Join(base, "zapretstat", "zstats.jsonl")
}

func loadData(path string) ([]Sample, []Event, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, nil, err
	}
	defer f.Close()

	var samples []Sample
	var events []Event
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		var raw struct {
			Type string `json:"type"`
		}
		if json.Unmarshal(line, &raw) != nil {
			continue
		}

		switch raw.Type {
		case "sample":
			var s Sample
			if json.Unmarshal(line, &s) == nil {
				samples = append(samples, s)
			}
		case "event":
			var e Event
			if json.Unmarshal(line, &e) == nil {
				events = append(events, e)
			}
		}
	}

	sap := []Sample{}
	for _, s := range samples {
		if time.Since(s.Timestamp).Hours() <= 24 {
			sap = append(sap, s)
		}
	}

	return sap, events, scanner.Err()
}

type model struct {
	tslc    timeserieslinechart.Model
	samples []Sample
	events  []Event
	lastMod time.Time
}

func (m model) Init() tea.Cmd {
	return tea.Every(2*time.Second, func(t time.Time) tea.Msg { return t })
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		case "r":
			m.tslc.ClearAllData()
			return m, nil
		}

	case tea.WindowSizeMsg:
		m.tslc.Resize(msg.Width, msg.Height-8)
		return m, nil

	case time.Time:
		path := logFilePath()
		if info, err := os.Stat(path); err == nil && !info.ModTime().Equal(m.lastMod) {
			m.lastMod = info.ModTime()
			samples, events, _ := loadData(path)
			m.samples = samples
			m.events = events

			m.tslc.ClearAllData()
			for _, s := range samples {
				latency := float64(s.LatencyMS)
				if !s.Internet {
					latency = 0
				}
				m.tslc.Push(timeserieslinechart.TimePoint{
					Time:  s.Timestamp,
					Value: latency,
				})
			}
		}

		m.tslc.DrawBraille()
		m.addMarkers()
	}

	return m, nil
}

func (m *model) addMarkers() {
	flagStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("196"))
	eventStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("226"))

	// Только флаги (без вертикальных черточек)
	for _, s := range m.samples {
		if !s.Internet {
			x := m.mapDataToCanvasX(s.Timestamp)
			y := m.mapDataToCanvasY(45) // чуть выше нуля

			p := canvas.Point{X: x, Y: y - 1}
			if p.X >= 0 && p.X < m.tslc.Canvas.Width() && p.Y >= 0 && p.Y < m.tslc.Canvas.Height() {
				m.tslc.Canvas.SetRuneWithStyle(p, '⚑', flagStyle)
			}
		}
	}

	for _, e := range m.events {
		x := m.mapDataToCanvasX(e.Timestamp)
		p := canvas.Point{X: x, Y: 1}
		if p.X >= 0 && p.X < m.tslc.Canvas.Width() {
			m.tslc.Canvas.SetRuneWithStyle(p, '!', eventStyle)
		}
	}
}

func (m model) mapDataToCanvasX(t time.Time) int {
	minX, maxX := m.tslc.ViewMinX(), m.tslc.ViewMaxX()
	if maxX == minX {
		return m.tslc.Origin().X + m.tslc.GraphWidth()/2
	}
	normalized := (float64(t.UnixMilli())/1000.0 - minX) / (maxX - minX)
	return m.tslc.Origin().X + int(normalized*float64(m.tslc.GraphWidth()))
}

func (m model) mapDataToCanvasY(value float64) int {
	minY, maxY := m.tslc.ViewMinY(), m.tslc.ViewMaxY()
	if maxY == minY {
		return m.tslc.Origin().Y
	}
	normalized := (value - minY) / (maxY - minY)
	return m.tslc.Origin().Y - int(normalized*float64(m.tslc.GraphHeight()))
}

func (m model) View() tea.View {
	header := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("12")).
		Render("📈 Zapretstat Latency | q — выход | r — очистка | обновляется каждые 2с")

	footer := fmt.Sprintf("Сэмплов: %d | Событий: %d", len(m.samples), len(m.events))

	content := header + "\n\n" + m.tslc.View() + "\n\n" + footer
	v := tea.NewView(content)
	v.AltScreen = true
	return v
}

func main() {
	path := logFilePath()
	samples, events, err := loadData(path)
	if err != nil {
		fmt.Println("Ошибка загрузки:", err)
		return
	}

	tslc := timeserieslinechart.New(
		130, 38,
		timeserieslinechart.WithXLabelFormatter(timeserieslinechart.HourTimeLabelFormatter()),
		timeserieslinechart.WithXYSteps(12, 6), // больше меток времени
	)

	// Начальное заполнение
	for _, s := range samples {
		latency := float64(s.LatencyMS)
		if !s.Internet {
			latency = 0
		}
		tslc.Push(timeserieslinechart.TimePoint{Time: s.Timestamp, Value: latency})
	}

	tslc.DrawBraille()

	m := model{
		tslc:    tslc,
		samples: samples,
		events:  events,
	}

	if _, err := tea.NewProgram(m).Run(); err != nil {
		fmt.Println("Ошибка запуска:", err)
	}
}
