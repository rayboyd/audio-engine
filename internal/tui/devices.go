package tui

import (
	"fmt"
	"strings"

	"audio/internal/audio"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

var (
	titleStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FFFDF5")).
			Background(lipgloss.Color("#25A065")).
			Padding(0, 1).
			Bold(true)

	infoStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FFFDF5"))

	highlightStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#25A065")).
			Bold(true)
)

// ScreenType defines which screen is currently active
type ScreenType int

const (
	ListScreen ScreenType = iota
	ConfigScreen
)

// DeviceListModel represents the Bubble Tea model for listing audio devices
type DeviceListModel struct {
	devices       []audio.Device
	selectedIndex int
	viewport      viewport.Model
	ready         bool
	err           error
	activeScreen  ScreenType

	// Configuration options
	selectedSampleRate   float64
	availableSampleRates []float64
	sampleRateIndex      int
}

// Init initializes the Bubble Tea model
func (m DeviceListModel) Init() tea.Cmd {
	return fetchDevices
}

// fetchDevices gets the available audio devices
func fetchDevices() tea.Msg {
	devices, err := audio.GetDevices()
	if err != nil {
		return errMsg{err}
	}
	return devicesMsg{devices}
}

// Update handles input and updates the model
type devicesMsg struct {
	devices []audio.Device
}

type errMsg struct {
	err error
}

func (m DeviceListModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var (
		cmd  tea.Cmd
		cmds []tea.Cmd
	)

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		// Initialize the viewport with the window size
		if !m.ready {
			m.viewport = viewport.New(msg.Width, msg.Height-4)
			m.viewport.Style = lipgloss.NewStyle()
			m.ready = true

			// If we already have devices, render them now
			if len(m.devices) > 0 {
				m.viewport.SetContent(m.renderDevices())
			}
		} else {
			// Just update viewport dimensions
			m.viewport.Width = msg.Width
			m.viewport.Height = msg.Height - 4
		}

	case devicesMsg:
		m.devices = msg.devices
		if m.ready {
			m.viewport.SetContent(m.renderDevices())
		}

	case errMsg:
		m.err = msg.err

	case tea.KeyMsg:
		// First check for keys that should work everywhere
		if key.Matches(msg, key.NewBinding(key.WithKeys("q", "ctrl+c"))) {
			return m, tea.Quit
		}

		// Then handle screen-specific keys
		if m.activeScreen == ListScreen {
			switch {
			case key.Matches(msg, key.NewBinding(key.WithKeys("up", "k"))):
				if m.selectedIndex > 0 {
					m.selectedIndex--
					m.viewport.SetContent(m.renderDevices())
				}

			case key.Matches(msg, key.NewBinding(key.WithKeys("down", "j"))):
				if m.selectedIndex < len(m.devices)-1 {
					m.selectedIndex++
					m.viewport.SetContent(m.renderDevices())
				}

			case key.Matches(msg, key.NewBinding(key.WithKeys("enter"))):
				if len(m.devices) > 0 {
					// Switch to configuration screen
					m.activeScreen = ConfigScreen

					// Set up sample rates
					m.availableSampleRates = []float64{44100, 48000, 88200, 96000}
					m.selectedSampleRate = m.devices[m.selectedIndex].DefaultSampleRate

					// Find the index of the default sample rate
					m.sampleRateIndex = 0
					for i, rate := range m.availableSampleRates {
						if rate == m.selectedSampleRate {
							m.sampleRateIndex = i
							break
						}
					}

					// Update viewport to show configuration
					m.viewport.SetContent(m.renderDeviceConfig())
				}
			}
		} else if m.activeScreen == ConfigScreen {
			switch {
			case key.Matches(msg, key.NewBinding(key.WithKeys("esc"))):
				// Return to list screen
				m.activeScreen = ListScreen
				m.viewport.SetContent(m.renderDevices())

			case key.Matches(msg, key.NewBinding(key.WithKeys("up", "k"))):
				if m.sampleRateIndex > 0 {
					m.sampleRateIndex--
					m.selectedSampleRate = m.availableSampleRates[m.sampleRateIndex]
					m.viewport.SetContent(m.renderDeviceConfig())
				}

			case key.Matches(msg, key.NewBinding(key.WithKeys("down", "j"))):
				if m.sampleRateIndex < len(m.availableSampleRates)-1 {
					m.sampleRateIndex++
					m.selectedSampleRate = m.availableSampleRates[m.sampleRateIndex]
					m.viewport.SetContent(m.renderDeviceConfig())
				}
			}
		}

		// Then handle screen-specific keys
		// Rest of your key handling code remains the same...
	}

	// Handle viewport updates
	m.viewport, cmd = m.viewport.Update(msg)
	cmds = append(cmds, cmd)

	return m, tea.Batch(cmds...)
}

// View renders the UI
func (m DeviceListModel) View() string {
	if !m.ready {
		return "Initializing..."
	}

	if m.err != nil {
		return fmt.Sprintf("Error: %v\n\nPress any key to exit.", m.err)
	}

	var title, help string

	if m.activeScreen == ListScreen {
		title = titleStyle.Render("Audio Device List")
		help = infoStyle.Render("↑/↓: Navigate • Enter: Configure • q: Quit")
	} else {
		title = titleStyle.Render("Device Configuration")
		help = infoStyle.Render("↑/↓: Change Value • Esc: Back • q: Quit")
	}

	return fmt.Sprintf("%s\n\n%s\n\n%s", title, m.viewport.View(), help)
}

// renderDevices formats the device list
func (m DeviceListModel) renderDevices() string {
	var sb strings.Builder

	if len(m.devices) == 0 {
		return "No audio devices found."
	}

	for i, device := range m.devices {
		// Determine device type
		deviceType := ""
		if device.MaxInputChannels > 0 && device.MaxOutputChannels > 0 {
			deviceType = "Input/Output"
		} else if device.MaxInputChannels > 0 {
			deviceType = "Input"
		} else if device.MaxOutputChannels > 0 {
			deviceType = "Output"
		}

		deviceInfo := fmt.Sprintf("[%d] %s (%s)\n",
			device.ID, device.Name, deviceType)
		deviceInfo += fmt.Sprintf("    Input channels: %d, Output channels: %d\n",
			device.MaxInputChannels, device.MaxOutputChannels)
		deviceInfo += fmt.Sprintf("    Default sample rate: %.0f Hz\n",
			device.DefaultSampleRate)

		if i == m.selectedIndex {
			deviceInfo = highlightStyle.Render(deviceInfo)
		}

		sb.WriteString(deviceInfo)
		sb.WriteString("\n")
	}

	return sb.String()
}

// renderDeviceConfig formats the device configuration screen
func (m DeviceListModel) renderDeviceConfig() string {
	var sb strings.Builder
	device := m.devices[m.selectedIndex]

	sb.WriteString(fmt.Sprintf("Configure Device: %s\n\n", device.Name))
	sb.WriteString("Sample Rate:\n")

	// Display sample rate options
	for i, rate := range m.availableSampleRates {
		line := fmt.Sprintf("  %s %.0f Hz\n",
			func() string {
				if i == m.sampleRateIndex {
					return "▶"
				}
				return " "
			}(),
			rate)

		if i == m.sampleRateIndex {
			line = highlightStyle.Render(line)
		}

		sb.WriteString(line)
	}

	// Add more configuration options here as needed

	return sb.String()
}

// NewDeviceListModel creates a new device list model
func NewDeviceListModel() DeviceListModel {
	return DeviceListModel{
		selectedIndex: 0,
		activeScreen:  ListScreen,
	}
}

// StartDeviceListUI launches the Bubble Tea TUI for listing devices
func StartDeviceListUI() error {
	p := tea.NewProgram(
		NewDeviceListModel(),
		tea.WithAltScreen(),
	)
	_, err := p.Run()
	return err
}
