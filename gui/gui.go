//go:build gui

package gui

import (
	"image/color"
	"sync"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"syscleaner/gui/views"
	"syscleaner/pkg/gaming"
)

// modernTheme implements a sleek dark theme with flame-orange accents.
// When extreme mode is active, it uses red accents instead.
type modernTheme struct {
	extremeModeActive bool
}

func (m *modernTheme) Color(name fyne.ThemeColorName, variant fyne.ThemeVariant) color.Color {
	// Check if extreme mode is active and adjust primary color
	primaryColor := color.RGBA{R: 255, G: 85, B: 0, A: 255} // Orange
	if gaming.IsExtremeModeActive() {
		primaryColor = color.RGBA{R: 220, G: 30, B: 30, A: 255} // Red
	}

	switch name {
	case theme.ColorNameBackground:
		return color.RGBA{R: 18, G: 18, B: 18, A: 255}
	case theme.ColorNameButton:
		return color.RGBA{R: 45, G: 45, B: 48, A: 255}
	case theme.ColorNamePrimary:
		return primaryColor
	case theme.ColorNameHover:
		if gaming.IsExtremeModeActive() {
			return color.RGBA{R: 255, G: 50, B: 50, A: 255} // Lighter red
		}
		return color.RGBA{R: 255, G: 110, B: 30, A: 255}
	case theme.ColorNameForeground:
		return color.RGBA{R: 230, G: 230, B: 230, A: 255}
	case theme.ColorNameDisabled:
		return color.RGBA{R: 100, G: 100, B: 100, A: 255}
	case theme.ColorNameInputBackground:
		return color.RGBA{R: 30, G: 30, B: 33, A: 255}
	case theme.ColorNameSeparator:
		return color.RGBA{R: 55, G: 55, B: 58, A: 255}
	default:
		return theme.DefaultTheme().Color(name, variant)
	}
}

func (m *modernTheme) Font(style fyne.TextStyle) fyne.Resource {
	return theme.DefaultTheme().Font(style)
}

func (m *modernTheme) Icon(name fyne.ThemeIconName) fyne.Resource {
	return theme.DefaultTheme().Icon(name)
}

func (m *modernTheme) Size(name fyne.ThemeSizeName) float32 {
	switch name {
	case theme.SizeNamePadding:
		return 6
	case theme.SizeNameInnerPadding:
		return 10
	case theme.SizeNameText:
		return 14
	case theme.SizeNameHeadingText:
		return 22
	case theme.SizeNameSubHeadingText:
		return 17
	default:
		return theme.DefaultTheme().Size(name)
	}
}

// Run launches the GUI application.
func Run() {
	a := app.NewWithID("com.syscleaner.app")
	customTheme := &modernTheme{}
	a.Settings().SetTheme(customTheme)

	w := a.NewWindow("SysCleaner - Ultimate Performance")
	w.Resize(fyne.NewSize(1200, 800))
	w.CenterOnScreen()
	w.SetMaster()

	mainContainer := createMainInterface(w)
	w.SetContent(mainContainer)
	w.ShowAndRun()
}

// lazyTab creates a tab whose content is built on first selection.
// This avoids initializing heavy panels (monitors, process lists) at startup.
func lazyTab(name string, icon fyne.Resource, builder func() fyne.CanvasObject) *container.TabItem {
	var once sync.Once
	placeholder := container.NewStack(widget.NewLabel("Loading..."))
	tab := container.NewTabItemWithIcon(name, icon, placeholder)

	// The actual content will be swapped in on first view via OnSelected.
	// We store the builder and once so createMainInterface can wire them up.
	tab.Content = &lazyContainer{
		placeholder: placeholder,
		builder:     builder,
		once:        &once,
	}
	return tab
}

// lazyContainer defers building its real content until first render.
type lazyContainer struct {
	widget.BaseWidget
	placeholder *fyne.Container
	builder     func() fyne.CanvasObject
	once        *sync.Once
	real        fyne.CanvasObject
}

func (l *lazyContainer) CreateRenderer() fyne.WidgetRenderer {
	l.once.Do(func() {
		l.real = l.builder()
	})
	if l.real != nil {
		return widget.NewSimpleRenderer(l.real)
	}
	return widget.NewSimpleRenderer(l.placeholder)
}

func createMainInterface(w fyne.Window) fyne.CanvasObject {
	// Dashboard loads eagerly since it's the first visible tab
	dashTab := container.NewTabItemWithIcon("Dashboard", theme.HomeIcon(), views.NewDashboard())

	// Other tabs load lazily on first selection
	extremeTab := lazyTab("Extreme Mode", theme.WarningIcon(), func() fyne.CanvasObject {
		return views.NewExtremeModePanel(w)
	})
	cleanTab := lazyTab("Clean", theme.DeleteIcon(), views.NewCleanPanel)
	optimizeTab := lazyTab("Optimize", theme.SettingsIcon(), views.NewOptimizePanel)
	cpuTab := lazyTab("CPU Priority", theme.MediaPlayIcon(), func() fyne.CanvasObject {
		return views.NewPriorityPanel(w)
	})
	monitorTab := lazyTab("Monitor", theme.InfoIcon(), views.NewMonitorPanel)

	tabs := container.NewAppTabs(dashTab, extremeTab, cleanTab, optimizeTab, cpuTab, monitorTab)
	tabs.SetTabLocation(container.TabLocationLeading)

	// Trigger lazy content initialization when a tab is selected
	tabs.OnSelected = func(item *container.TabItem) {
		if lc, ok := item.Content.(*lazyContainer); ok {
			lc.once.Do(func() {
				lc.real = lc.builder()
			})
			if lc.real != nil {
				item.Content = lc.real
				tabs.Refresh()
			}
		}
	}

	return tabs
}
