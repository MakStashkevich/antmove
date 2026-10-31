package main

import (
	"fmt"
	"os"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/mattn/go-runewidth"
	"github.com/muesli/termenv"
)

type tuiModel struct {
	page          string
	items         []Item
	operations    []Operation
	volumes       []Volume
	selectedItems map[string]bool
	selectedOps   map[string]bool
	cursor        int
	volumeCursor  int
	confirmStage  int
	confirmText   string
	search        string
	searching     bool
	busy          bool
	loading       bool
	loaded        bool
	spinner       int
	dryRun        bool
	message       string
	menuCursor    int
	width         int
	height        int
	returnPage    string
	quitAction    string
	antOffset     int
	antDirection  int
	antPause      int
	antWiggle     int
	antWalkFrame  int
	antHasBall    bool
	antBallDrop   int
}

type refreshMsg struct {
	items      []Item
	operations []Operation
	volumes    []Volume
	err        error
}

type operationMsg struct {
	message string
	page    string
}

type tickMsg time.Time

const maxAntTravelOffset = 20

var tuiRenderer = newTUIRenderer()

func newTUIRenderer() *lipgloss.Renderer {
	renderer := lipgloss.NewRenderer(os.Stdout, termenv.WithProfile(termenv.ANSI))
	renderer.SetColorProfile(termenv.ANSI)
	return renderer
}

var (
	tuiSelectedStyle = tuiRenderer.NewStyle().Foreground(lipgloss.Color("6")).Bold(true)
	tuiMutedStyle    = tuiRenderer.NewStyle().Foreground(lipgloss.Color("8"))
	tuiHeadingStyle  = tuiRenderer.NewStyle().Foreground(lipgloss.Color("5")).Bold(true)
	tuiBrandStyle    = tuiRenderer.NewStyle().Foreground(lipgloss.Color("6")).Bold(true)
	tuiTaglineStyle  = tuiRenderer.NewStyle().Foreground(lipgloss.Color("2"))
	tuiSuccessStyle  = tuiRenderer.NewStyle().Foreground(lipgloss.Color("2"))
	tuiWarningStyle  = tuiRenderer.NewStyle().Foreground(lipgloss.Color("1")).Bold(true)
	tuiActiveStyle   = tuiRenderer.NewStyle().Foreground(lipgloss.Color("6"))
)

func teaNewProgram(model tea.Model) *tea.Program {
	return tea.NewProgram(model, tea.WithAltScreen())
}

func newTUIModel(page string, dryRun bool) (tuiModel, error) {
	model := tuiModel{
		page: page, selectedItems: map[string]bool{}, selectedOps: map[string]bool{}, dryRun: dryRun,
		message: "Scanning candidates and disks…", loading: true, width: 80, height: 24, antDirection: 1, antBallDrop: 3,
	}
	if startupNotice != "" {
		model.message = startupNotice
	}
	return model, nil
}

func (m *tuiModel) load() error {
	items, err := scanItems()
	if err != nil {
		return err
	}
	operations, err := loadOperations()
	if err != nil {
		return err
	}
	volumes, err := scanVolumes()
	if err != nil {
		return err
	}
	m.items, m.operations, m.volumes = items, operations, volumes
	return nil
}

func (m tuiModel) Init() tea.Cmd {
	return tea.Batch(tickCommand(), refreshCommand())
}

func (m tuiModel) Update(message tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := message.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		return m, nil
	case refreshMsg:
		m.busy = false
		m.loading = false
		m.loaded = true
		if msg.err != nil {
			m.message = msg.err.Error()
			return m, nil
		}
		m.items, m.operations, m.volumes = msg.items, msg.operations, msg.volumes
		if m.message == "Scanning candidates and disks…" {
			m.message = ""
		}
		m.cursor, m.volumeCursor = 0, 0
		return m, nil
	case operationMsg:
		if msg.page == "uninstalled" {
			m.page, m.message, m.busy = "uninstalled", msg.message, false
			return m, nil
		}
		m.message = msg.message
		m.busy = false
		if msg.page != "" {
			m.page = msg.page
		} else if m.page == "moving" {
			m.page = "items"
		} else if m.page == "restoring" {
			m.page = "restore"
		}
		m.loading = true
		return m, tea.Batch(tickCommand(), refreshCommand())
	case tickMsg:
		if m.page == "home" {
			m.animateAnt()
			return m, tickCommand()
		}
		if m.busy || m.loading {
			m.spinner = (m.spinner + 1) % 4
			return m, tickCommand()
		}
		return m, nil
	case tea.KeyMsg:
		if msg.String() == "ctrl+c" {
			return m, tea.Quit
		}
		if m.busy {
			return m, nil
		}
		if m.searching {
			return m.updateSearch(msg)
		}
		previousPage := m.page
		model, command := m.updateKey(msg)
		if previousPage != "home" {
			if updated, ok := model.(tuiModel); ok && updated.page == "home" {
				command = tea.Batch(command, tickCommand())
			}
		}
		return model, command
	}
	return m, nil
}

func (m tuiModel) updateSearch(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.searching = false
	case "enter":
		m.searching = false
		m.cursor = 0
	case "backspace":
		runes := []rune(m.search)
		if len(runes) > 0 {
			m.search = string(runes[:len(runes)-1])
		}
	default:
		if len(msg.Runes) > 0 {
			m.search += string(msg.Runes)
		}
	}
	return m, nil
}

func (m tuiModel) updateKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()
	switch m.page {
	case "home":
		switch key {
		case "up", "k":
			m.menuCursor = maxInt(0, m.menuCursor-1)
		case "down", "j":
			m.menuCursor = minInt(7, m.menuCursor+1)
		case "enter":
			return m.activateHomeOption(m.menuCursor)
		case "1":
			return m.activateHomeOption(0)
		case "2":
			return m.activateHomeOption(1)
		case "3":
			return m.activateHomeOption(2)
		case "4":
			return m.activateHomeOption(3)
		case "5":
			return m.activateHomeOption(4)
		case "6":
			return m.activateHomeOption(5)
		case "7":
			return m.activateHomeOption(6)
		case "8":
			return m.activateHomeOption(7)
		case "m":
			m.quitAction = "help"
			return m, tea.Quit
		case "v":
			m.quitAction = "version"
			return m, tea.Quit
		case "q", "esc":
			return m, tea.Quit
		}
	case "items":
		count := len(m.filteredItems())
		switch key {
		case "up", "k":
			m.cursor = maxInt(0, m.cursor-1)
		case "down", "j":
			m.cursor = minInt(maxInt(0, count-1), m.cursor+1)
		case " ", "space", "enter":
			m.toggleCurrentItem()
		case "a":
			for _, item := range m.filteredItems() {
				if item.Status == StatusAvailable {
					m.selectedItems[item.ID] = true
				}
			}
		case "n":
			m.selectedItems = map[string]bool{}
		case "m":
			if len(m.selectedItems) == 0 {
				m.message = "Select at least one available item first."
			} else {
				m.page, m.volumeCursor = "disks", 0
			}
		case "d":
			m.page, m.volumeCursor = "disks", 0
		case "r":
			m.page, m.cursor = "restore", 0
		case "s":
			m.page = "status"
		case "/":
			m.searching, m.search = true, ""
		case "esc", "h":
			m.page, m.search = "home", ""
		case "q":
			return m, tea.Quit
		}
	case "disks":
		switch key {
		case "up", "k":
			m.volumeCursor = maxInt(0, m.volumeCursor-1)
		case "down", "j":
			m.volumeCursor = minInt(maxInt(0, len(m.volumes)-1), m.volumeCursor+1)
		case "enter":
			if len(m.volumes) == 0 {
				m.message = "No external disks are available."
				break
			}
			volume := m.volumes[m.volumeCursor]
			if volume.Rating == RatingUnavailable {
				m.message = "This disk cannot be used as a destination."
			} else if len(m.selectedItems) == 0 {
				m.message = "Select items first, then press m to move them."
			} else {
				m.confirmStage, m.confirmText = 0, ""
				m.page = "confirm"
			}
		case "t":
			if len(m.volumes) > 0 {
				m.busy = true
				m.message = "Measuring read/write speed."
				path := m.volumes[m.volumeCursor].MountPath
				return m, tea.Batch(tickCommand(), func() tea.Msg {
					result, err := testVolume(path)
					if err != nil {
						return operationMsg{message: "Speed test failed: " + err.Error(), page: "disks"}
					}
					return operationMsg{message: fmt.Sprintf("Speed test complete: read %.0f MB/s, write %.0f MB/s", result.ReadMBps, result.WriteMBps), page: "disks"}
				})
			}
		case "esc", "h":
			m.page = "home"
		case "q":
			return m, tea.Quit
		}
	case "confirm":
		return m.updateConfirmation(msg)
	case "restore":
		eligible := m.restorableOperations()
		switch key {
		case "up", "k":
			m.cursor = maxInt(0, m.cursor-1)
		case "down", "j":
			m.cursor = minInt(maxInt(0, len(eligible)-1), m.cursor+1)
		case " ", "space", "enter":
			if len(eligible) > 0 {
				op := eligible[m.cursor]
				m.selectedOps[op.ID] = !m.selectedOps[op.ID]
			}
		case "a":
			for _, op := range eligible {
				m.selectedOps[op.ID] = true
			}
		case "n":
			m.selectedOps = map[string]bool{}
		case "r":
			if len(m.selectedOps) == 0 {
				m.message = "Select at least one moved item first."
			} else {
				if m.dryRun {
					m.message = "Dry run: selected items are ready to restore."
				} else {
					m.page, m.confirmStage = "restore-confirm", 0
				}
			}
		case "esc", "h":
			m.page = "home"
		case "q":
			return m, tea.Quit
		}
	case "restore-confirm":
		switch key {
		case "y", "enter":
			return m.startRestore()
		case "n", "esc":
			m.page = "restore"
			m.message = "Restore cancelled."
		}
	case "status":
		switch key {
		case "up", "k":
			m.cursor = maxInt(0, m.cursor-1)
		case "down", "j":
			m.cursor = minInt(maxInt(0, len(m.operations)-1), m.cursor+1)
		case "r":
			m.returnPage = "status"
			m.page = "recover-confirm"
		case "esc", "h":
			m.page = "home"
		case "q":
			return m, tea.Quit
		}
	case "recover-confirm":
		switch key {
		case "y", "enter":
			m.busy = true
			m.message = "Recovering interrupted transfers."
			returnPage := nonempty(m.returnPage, "status")
			return m, tea.Batch(tickCommand(), func() tea.Msg {
				unlock, err := acquireLock()
				if err != nil {
					return operationMsg{message: err.Error(), page: returnPage}
				}
				defer unlock()
				results, err := recoverOperations()
				if err != nil {
					return operationMsg{message: "Recovery failed: " + err.Error(), page: returnPage}
				}
				if len(results) == 0 {
					return operationMsg{message: "No interrupted operations need recovery.", page: returnPage}
				}
				return operationMsg{message: strings.Join(results, "\n"), page: returnPage}
			})
		case "n", "esc":
			m.page, m.message = nonempty(m.returnPage, "status"), "Recovery cancelled."
		}
	case "uninstall-confirm":
		return m.updateUninstallConfirmation(msg)
	case "uninstalling":
		if key == "ctrl+c" {
			return m, nil
		}
	case "uninstalled":
		return m, tea.Quit
	}
	return m, nil
}

func (m tuiModel) activateHomeOption(option int) (tea.Model, tea.Cmd) {
	m.menuCursor = option
	m.cursor = 0
	m.message = ""
	switch option {
	case 0:
		m.page = "items"
	case 1:
		m.page = "items"
		m.message = "Candidate scan complete. Select items with Space."
	case 2:
		m.page = "restore"
	case 3:
		m.page = "disks"
	case 4:
		m.page = "status"
	case 5:
		m.returnPage = "home"
		m.page = "recover-confirm"
	case 6:
		installed, pathChanged, err := installSelf()
		if err != nil {
			m.message = "Install failed: " + err.Error()
		} else if installed {
			m.message = "Installed antmove and am in " + installDirectoryLabel()
		} else {
			m.message = "antmove and am are already installed in " + installDirectoryLabel()
		}
		if pathChanged {
			m.message += ". Reopen Terminal to refresh PATH."
		}
	case 7:
		m.page, m.confirmStage, m.confirmText = "uninstall-confirm", 0, ""
	default:
		return m, nil
	}
	return m, nil
}

func (m tuiModel) updateUninstallConfirmation(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()
	if key == "esc" || key == "n" {
		m.page, m.message = "home", "Uninstall cancelled."
		return m, nil
	}
	if m.confirmStage == 0 {
		if key == "y" || key == "enter" {
			m.confirmStage = 1
		}
		return m, nil
	}
	if key == "backspace" {
		runes := []rune(m.confirmText)
		if len(runes) > 0 {
			m.confirmText = string(runes[:len(runes)-1])
		}
		return m, nil
	}
	if key == "enter" {
		if m.confirmText != uninstallConfirmation {
			m.confirmText = ""
			m.message = "Confirmation did not match. Uninstall cancelled."
			m.page = "home"
			return m, nil
		}
		m.page, m.busy = "uninstalling", true
		m.message = "Removing antmove and its state."
		return m, tea.Batch(tickCommand(), func() tea.Msg {
			if err := performUninstall(); err != nil {
				return operationMsg{message: "Uninstall failed: " + err.Error(), page: "home"}
			}
			return operationMsg{message: "antmove and its data were removed. Press any key to exit.", page: "uninstalled"}
		})
	}
	if len(msg.Runes) > 0 {
		m.confirmText += string(msg.Runes)
	}
	return m, nil
}

func (m tuiModel) updateConfirmation(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()
	if key == "esc" || key == "n" {
		m.page, m.message = "disks", "Move cancelled."
		return m, nil
	}
	if len(m.volumes) == 0 || m.volumeCursor >= len(m.volumes) {
		m.page = "disks"
		return m, nil
	}
	volume := m.volumes[m.volumeCursor]
	if volume.Rating != RatingCaution {
		if key == "enter" || key == "y" {
			return m.startMove(volume)
		}
		return m, nil
	}
	if m.confirmStage == 0 {
		if key == "enter" || key == "y" {
			m.confirmStage = 1
			m.message = "Second confirmation required: type the exact disk name below."
		}
		return m, nil
	}
	if key == "enter" {
		if m.confirmText == volume.Name {
			return m.startMove(volume)
		}
		m.confirmText, m.message = "", "Disk name did not match; try again or press Esc."
		return m, nil
	}
	if key == "backspace" {
		runes := []rune(m.confirmText)
		if len(runes) > 0 {
			m.confirmText = string(runes[:len(runes)-1])
		}
		return m, nil
	}
	if len(msg.Runes) > 0 {
		m.confirmText += string(msg.Runes)
	}
	return m, nil
}

func (m tuiModel) startMove(volume Volume) (tea.Model, tea.Cmd) {
	items := m.selectedItemList()
	if len(items) == 0 {
		m.page, m.message = "items", "No eligible items selected."
		return m, nil
	}
	m.busy, m.page = true, "moving"
	m.message = "Preparing transfer…"
	dryRun := m.dryRun
	return m, tea.Batch(tickCommand(), func() tea.Msg {
		if !dryRun {
			unlock, err := acquireLock()
			if err != nil {
				return operationMsg{message: err.Error()}
			}
			defer unlock()
		}
		var lines []string
		for _, item := range items {
			op, err := moveItem(item, volume, dryRun)
			if err != nil {
				lines = append(lines, "FAILED "+item.Name+": "+err.Error())
			} else if dryRun {
				lines = append(lines, "PREVIEW "+item.Name+" → "+op.DestinationPath)
			} else {
				lines = append(lines, "MOVED "+item.Name+" → "+op.DestinationPath)
			}
		}
		return operationMsg{message: strings.Join(lines, "\n")}
	})
}

func (m tuiModel) startRestore() (tea.Model, tea.Cmd) {
	operations := m.selectedOperationList()
	m.busy, m.page = true, "restoring"
	m.message = "Restoring selected data."
	return m, tea.Batch(tickCommand(), func() tea.Msg {
		unlock, err := acquireLock()
		if err != nil {
			return operationMsg{message: err.Error()}
		}
		defer unlock()
		var results []string
		for _, op := range operations {
			if err := restoreOperation(op); err != nil {
				results = append(results, "FAILED "+op.Name+": "+err.Error())
			} else {
				results = append(results, "RESTORED "+op.Name)
			}
		}
		return operationMsg{message: strings.Join(results, "\n")}
	})
}

func refreshCommand() tea.Cmd {
	return func() tea.Msg {
		items, err := scanItems()
		if err != nil {
			return refreshMsg{err: err}
		}
		ops, err := loadOperations()
		if err != nil {
			return refreshMsg{err: err}
		}
		volumes, err := scanVolumes()
		return refreshMsg{items: items, operations: ops, volumes: volumes, err: err}
	}
}

func tickCommand() tea.Cmd {
	return tea.Tick(180*time.Millisecond, func(t time.Time) tea.Msg { return tickMsg(t) })
}

func (m *tuiModel) animateAnt() {
	width := m.width
	if width < 20 {
		width = 80
	}
	maxOffset := antMaxOffset(width)
	if m.antOffset > maxOffset {
		m.antOffset = maxOffset
	}
	if m.antBallDrop > 0 {
		m.antBallDrop--
		if m.antBallDrop == 0 {
			m.antHasBall = true
		}
		return
	}
	if m.antPause > 0 {
		m.antPause--
		m.antWiggle = m.antWiggle%2 + 1
		if m.antPause == 0 {
			m.antDirection *= -1
			m.antWiggle = 0
			m.antWalkFrame = 0
		}
		return
	}
	if m.antDirection > 0 && m.antOffset >= maxOffset {
		m.antHasBall = false
		m.antPause = 8
		m.antWiggle = 1
		return
	}
	if m.antDirection < 0 && m.antOffset <= 0 {
		m.antDirection = 1
		m.antBallDrop = 3
		m.antWalkFrame = 0
		return
	}
	m.antOffset += m.antDirection
	m.antWalkFrame = (m.antWalkFrame + 1) % 2
}

func (m tuiModel) animatedAntLogo() []string {
	logo := append([]string(nil), antLogoFacingLeft...)
	if m.antPause > 0 {
		if m.antWiggle%2 == 1 {
			logo[0] = `\  /`
		} else {
			logo[0] = `/  \`
		}
	} else if m.antWalkFrame == 1 {
		logo[len(logo)-1] = `   ^^  \\ //`
	}
	if m.antDirection > 0 {
		logo = mirrorASCIIArt(logo)
	}
	frame := []string{"", logo[0], logo[1], logo[2], logo[3]}
	if m.antDirection > 0 && m.antBallDrop > 0 {
		if m.antBallDrop == 3 {
			frame[0] = placeAntBall(frame[0], antBallColumn(logo))
		} else {
			frame[1] = placeAntBall(frame[1], antBallColumn(logo))
		}
	} else if m.antDirection > 0 && m.antHasBall {
		frame[1] = placeAntBall(frame[1], antBallColumn(logo))
	}
	return frame
}

func antBallColumn(logo []string) int {
	if len(logo) < 2 {
		return 0
	}
	if dots := strings.Index(logo[1], "..."); dots >= 0 {
		return dots + 1
	}
	return 0
}

func placeAntBall(line string, column int) string {
	characters := []rune(line)
	for len(characters) <= column {
		characters = append(characters, ' ')
	}
	characters[column] = '●'
	return string(characters)
}

func (m tuiModel) View() string {
	var b strings.Builder
	width := m.width
	if width < 20 {
		width = 80
	}
	logo := antLogo
	antOffset := 0
	if m.page == "home" {
		logo = m.animatedAntLogo()
		antOffset = m.antOffset
	}
	textStart := antHeaderTextStart(width)
	for i := range logo {
		b.WriteString(animatedBrandHeaderLineFor(logo, i, antOffset, textStart) + "\n")
	}
	b.WriteString(strings.Repeat("─", antHeaderWidth(width)) + "\n")
	switch m.page {
	case "home":
		b.WriteString("\n  Choose a command\n\n")
		options := []string{
			"Move data",
			"Scan candidates",
			"Restore moved data",
			"External disks",
			"Operation status",
			"Recover interrupted transfer",
			"Install or repair commands",
			"Uninstall antmove",
		}
		shortcuts := []string{"am move", "am scan", "am restore", "am disks", "am status", "am recover", "am install", "am uninstall"}
		for i, option := range options {
			marker := "  "
			if i == m.menuCursor {
				marker = "› "
			}
			b.WriteString(fmt.Sprintf("%s%d  %-34s %s\n", marker, i+1, option, shortcuts[i]))
		}
		if m.loaded {
			movedItems, movedBytes := completedTransferStats(m.operations)
			b.WriteString(fmt.Sprintf("\n  %d candidates · %d moved items · %d eligible disks\n", len(m.items), movedItems, eligibleVolumeCount(m.volumes)))
			b.WriteString(fmt.Sprintf("\n  Storage freedom · %s freed on this Mac and moved to external disks\n", formatBytes(movedBytes)))
		} else {
			b.WriteString(fmt.Sprintf("\n  %s Scanning candidates and disks…\n", spinnerFrame(m.spinner)))
		}
	case "items":
		available := 0
		for _, item := range m.filteredItems() {
			if item.Status == StatusAvailable {
				available++
			}
		}
		if m.loaded {
			b.WriteString(fmt.Sprintf("\n  Scan · %d candidates · %d available · %d selected\n\n", len(m.filteredItems()), available, len(m.selectedItems)))
		} else {
			b.WriteString(fmt.Sprintf("\n  %s Scanning candidates…\n\n", spinnerFrame(m.spinner)))
		}
		items := m.filteredItems()
		if len(items) == 0 && m.loaded {
			b.WriteString("  No candidates found.\n")
		}
		start, end := visibleRange(m.cursor, len(items), maxInt(1, m.height-14))
		for i := start; i < end; i++ {
			item := items[i]
			marker := rowMarker(i == m.cursor, item.Status == StatusAvailable && m.selectedItems[item.ID])
			b.WriteString(fmt.Sprintf("%s%-23s %-20s %-14s %9s\n", marker, truncate(item.Name, 22), item.Category, item.Status, formatBytes(item.SizeBytes)))
			if i == m.cursor {
				b.WriteString("    " + item.SourcePath + "\n")
				if item.DestinationPath != "" && item.Status == StatusMoved {
					b.WriteString("    External: " + item.DestinationPath + "\n")
				}
				if item.Owner != "" {
					b.WriteString("    App: " + item.Owner + "\n")
				}
				if item.Reason != "" {
					b.WriteString("    " + item.Reason + "\n")
				}
			}
		}
		b.WriteString(fmt.Sprintf("\n  Selected: %d    Filter: %s\n", len(m.selectedItems), nonempty(m.search, "none")))
	case "disks":
		b.WriteString("\n  External disks · Choose a destination\n\n")
		if !m.loaded {
			b.WriteString(fmt.Sprintf("  %s Scanning external disks…\n", spinnerFrame(m.spinner)))
		}
		if len(m.volumes) == 0 && m.loaded {
			b.WriteString("  No eligible physical external disks found.\n")
		}
		start, end := visibleRange(m.volumeCursor, len(m.volumes), maxInt(1, (m.height-10)/4))
		for i := start; i < end; i++ {
			volume := m.volumes[i]
			marker := "  "
			if i == m.volumeCursor {
				marker = "› "
			}
			b.WriteString(fmt.Sprintf("%s%s\n", marker, volumeMessage(volume)))
			b.WriteString("    Games: " + gameSuitability(volume) + " · Apps: " + appSuitability(volume) + " · Messengers: " + messengerSuitability(volume) + "\n")
			if volume.Rating == RatingCaution {
				b.WriteString("    Warning: this format may lose macOS metadata. Move requires two confirmations.\n")
			}
		}
	case "confirm":
		if len(m.volumes) > m.volumeCursor {
			volume := m.volumes[m.volumeCursor]
			b.WriteString("\n  Confirm transfer\n\n")
			b.WriteString(fmt.Sprintf("  Destination: %s (%s)\n  Selected items: %d\n", volume.Name, volume.Rating, len(m.selectedItems)))
			if volume.Rating == RatingCaution {
				b.WriteString("\n  WARNING: this disk may be slow or may not preserve macOS metadata.\n")
				if m.confirmStage == 0 {
					b.WriteString("\n  Continue only if you accept the disk compatibility warning.\n")
				} else {
					b.WriteString("\n  Type the disk name exactly to confirm.\n  > " + m.confirmText + "█\n")
				}
			} else {
				b.WriteString("\n  Ready to begin the transfer.\n")
			}
		}
	case "moving", "restoring":
		activity := "Moving selected data…"
		if m.page == "restoring" {
			activity = "Restoring selected data…"
		}
		b.WriteString(fmt.Sprintf("\n  %s %s\n\n  Do not disconnect the destination disk.\n", spinnerFrame(m.spinner), activity))
	case "restore":
		if m.loaded {
			b.WriteString(fmt.Sprintf("\n  Restore · %d available · %d selected\n\n", len(m.restorableOperations()), len(m.selectedOps)))
		} else {
			b.WriteString(fmt.Sprintf("\n  %s Loading operation history…\n\n", spinnerFrame(m.spinner)))
		}
		ops := m.restorableOperations()
		if len(ops) == 0 && m.loaded {
			b.WriteString("  No moved items are ready to restore.\n")
		}
		start, end := visibleRange(m.cursor, len(ops), maxInt(1, m.height-8))
		for i := start; i < end; i++ {
			op := ops[i]
			marker := rowMarker(i == m.cursor, m.selectedOps[op.ID])
			volume := findOperationVolume(m.volumes, op)
			status := "disk offline"
			if volume != nil {
				status = "disk connected"
			}
			b.WriteString(fmt.Sprintf("%s%-24s %9s · %s\n", marker, truncate(op.Name, 23), formatBytes(op.SizeBytes), status))
		}
	case "restore-confirm":
		b.WriteString("\n  Restore selected data\n\n  Data is copied back and verified before the external copy is removed.\n")
	case "status":
		if m.loaded {
			b.WriteString(fmt.Sprintf("\n  Status · %d recorded transfers\n\n", len(m.operations)))
		} else {
			b.WriteString(fmt.Sprintf("\n  %s Loading operation history…\n\n", spinnerFrame(m.spinner)))
		}
		start, end := visibleRange(m.cursor, len(m.operations), maxInt(1, (m.height-10)/2))
		if len(m.operations) == 0 && m.loaded {
			b.WriteString("  No transfers have been recorded.\n")
		}
		for i := start; i < end; i++ {
			op := m.operations[i]
			state := op.Phase
			if op.Phase == "completed" && findOperationVolume(m.volumes, op) == nil {
				state = "disk offline"
			}
			marker := "  "
			if i == m.cursor {
				marker = "› "
			}
			b.WriteString(fmt.Sprintf("%s%-18s %-24s %9s\n", marker, state, truncate(op.Name, 23), formatBytes(op.SizeBytes)))
			if i == m.cursor {
				b.WriteString("    Source: " + op.SourcePath + "\n")
				b.WriteString("    Destination: " + op.DestinationPath + "\n")
				if op.Origin != "" {
					b.WriteString("    Imported from: " + op.Origin + "\n")
				}
			}
			if op.Error != "" && i == m.cursor {
				b.WriteString("    " + op.Error + "\n")
			}
		}
	case "recover-confirm":
		if m.busy {
			b.WriteString(fmt.Sprintf("\n  %s Recovering interrupted transfers…\n", spinnerFrame(m.spinner)))
		} else {
			b.WriteString("\n  Recover interrupted transfers? Safe temporary copies will be cleaned or\n  original paths restored when the journal proves that is safe.\n")
		}
	case "uninstall-confirm":
		b.WriteString("\n  WARNING: antmove and its operation history will be permanently removed.\n  Moved apps will not be restored; their original paths remain symbolic links.\n  Keep their disks connected or restore them before uninstalling.\n\n")
		if m.confirmStage == 0 {
			b.WriteString("  First confirmation required.\n")
		} else {
			b.WriteString(fmt.Sprintf("  Type %q to confirm.\n  > %s█\n", uninstallConfirmation, m.confirmText))
		}
	case "uninstalling":
		b.WriteString(fmt.Sprintf("\n  %s Removing antmove and its state…\n", spinnerFrame(m.spinner)))
	case "uninstalled":
		b.WriteString("\n  " + m.message + "\n")
	}
	if m.searching {
		b.WriteString("\n  Filter: " + m.search + "█\n")
	}
	messageInBody := m.page == "moving" || m.page == "restoring" || (m.page == "recover-confirm" && m.busy) || m.page == "uninstalling"
	if m.message != "" && m.page != "uninstalled" && !messageInBody && !(m.message == "Scanning candidates and disks…" && !m.loaded) {
		b.WriteString("\n  " + strings.ReplaceAll(m.message, "\n", "\n  ") + "\n")
	}
	b.WriteString("\n" + strings.Repeat("─", width) + "\n")
	b.WriteString("  " + m.controlsLine() + "\n")
	return styleTerminalView(wrapTerminalView(b.String(), width), m.page, logo, antOffset, textStart)
}

func styleTerminalView(view, page string, logo []string, antOffset, textStart int) string {
	lines := strings.Split(view, "\n")
	footerStart := -1
	for i, line := range lines {
		if line != "" && strings.Trim(line, "─") == "" {
			footerStart = i + 1
		}
	}
	headingPending := false
	for i, line := range lines {
		switch {
		case i < len(logo):
			lines[i] = styledAnimatedBrandHeaderLineFor(logo, i, antOffset, textStart)
			if i == len(logo)-1 {
				headingPending = true
			}
		case page == "home" && strings.Contains(line, "Restore moved data"):
			if strings.HasPrefix(line, "› ") {
				lines[i] = tuiSelectedStyle.Render(line)
			}
		case strings.Contains(line, " · disk connected") || strings.Contains(line, " · disk offline"):
			lines[i] = styleRestoreDiskStatusLine(line)
		case headingPending && strings.TrimSpace(line) != "" && strings.Trim(line, "─") != "":
			lines[i] = tuiHeadingStyle.Render(line)
			headingPending = false
		case strings.Contains(line, "WARNING:") || strings.Contains(line, "Warning:") || strings.Contains(line, "FAILED") || strings.Contains(line, "unavailable") || strings.Contains(line, "disk offline") || strings.Contains(line, "not recommended"):
			lines[i] = tuiWarningStyle.Render(line)
		case strings.Contains(line, "MOVED ") || strings.Contains(line, "RESTORED ") || strings.Contains(line, "complete") || strings.Contains(line, "Storage freedom") || strings.Contains(line, "Excellent"):
			lines[i] = tuiSuccessStyle.Render(line)
		case strings.Contains(line, "Scanning") || strings.Contains(line, "Loading") || strings.Contains(line, "Recovering") || strings.Contains(line, "Measuring"):
			lines[i] = tuiActiveStyle.Render(line)
		case strings.HasPrefix(line, "› ") || strings.HasPrefix(line, " ● "):
			lines[i] = tuiSelectedStyle.Render(line)
		case strings.Trim(line, "─") == "" && line != "":
			lines[i] = tuiMutedStyle.Render(line)
		case i >= footerStart && line != "":
			lines[i] = tuiMutedStyle.Render(line)
		}
	}
	return strings.Join(lines, "\n")
}

func styleRestoreDiskStatusLine(line string) string {
	separator := strings.LastIndex(line, " · ")
	if separator < 0 {
		return line
	}
	prefix, status := line[:separator], line[separator+len(" · "):]
	statusStyle := tuiWarningStyle
	if status == "disk connected" {
		statusStyle = tuiSuccessStyle
	}
	if strings.HasPrefix(prefix, "› ") || strings.HasPrefix(prefix, " ● ") {
		prefix = tuiSelectedStyle.Render(prefix)
	}
	return prefix + " · " + statusStyle.Render(status)
}

func (m tuiModel) controlsLine() string {
	if m.busy {
		if m.page == "moving" || m.page == "restoring" {
			return fmt.Sprintf("%s Working… · Ctrl+C stop · do not disconnect the destination disk", spinnerFrame(m.spinner))
		}
		return fmt.Sprintf("%s Working… · Ctrl+C stop", spinnerFrame(m.spinner))
	}
	if m.searching {
		return "Enter apply filter  ·  Esc cancel  ·  Backspace delete"
	}
	var controls string
	switch m.page {
	case "home":
		controls = "↑/↓ move  ·  Enter select  ·  M more  ·  V version  ·  Q quit"
	case "items":
		controls = "↑/↓ move  ·  Space select  ·  A all  ·  N none  ·  / filter  ·  M move  ·  D disks  ·  Esc back  ·  Q quit"
	case "disks":
		controls = "↑/↓ move  ·  Enter select  ·  T speed test  ·  Esc back  ·  Q quit"
	case "confirm":
		if m.confirmStage == 1 {
			controls = "Type disk name  ·  Enter confirm  ·  Esc cancel"
		} else {
			controls = "Y/Enter confirm  ·  Esc cancel"
		}
	case "restore":
		controls = "↑/↓ move  ·  Space select  ·  A all  ·  N none  ·  R restore  ·  Esc back  ·  Q quit"
	case "restore-confirm":
		controls = "Y/Enter restore  ·  Esc cancel"
	case "status":
		controls = "↑/↓ move  ·  R recover  ·  Esc back  ·  Q quit"
	case "recover-confirm":
		controls = "Y/Enter recover  ·  Esc cancel"
	case "uninstall-confirm":
		if m.confirmStage == 1 {
			controls = "Type confirmation  ·  Enter uninstall  ·  Esc cancel"
		} else {
			controls = "Y/Enter continue  ·  Esc cancel"
		}
	case "uninstalled":
		controls = "Press any key to exit"
	default:
		controls = "Q quit"
	}
	if m.loading && m.loaded {
		controls += fmt.Sprintf("  ·  %s refreshing", spinnerFrame(m.spinner))
	}
	return controls
}

func spinnerFrame(frame int) string {
	frames := [...]string{"⠋", "⠙", "⠹", "⠸"}
	return frames[frame%len(frames)]
}

func rowMarker(cursor, selected bool) string {
	switch {
	case cursor && selected:
		return "›● "
	case cursor:
		return "›  "
	case selected:
		return " ● "
	default:
		return "   "
	}
}

func wrapTerminalView(view string, width int) string {
	if width < 20 {
		width = 80
	}
	var wrapped strings.Builder
	for _, line := range strings.Split(strings.TrimSuffix(view, "\n"), "\n") {
		if runewidth.StringWidth(line) <= width {
			wrapped.WriteString(line)
			wrapped.WriteByte('\n')
			continue
		}
		indent := len(line) - len(strings.TrimLeft(line, " "))
		indent = minInt(indent, maxInt(0, width/3))
		continuation := strings.Repeat(" ", indent)
		available := width - runewidth.StringWidth(continuation)
		var current strings.Builder
		current.WriteString(continuation)
		currentWidth := runewidth.StringWidth(continuation)
		writeCurrent := func() {
			wrapped.WriteString(current.String())
			wrapped.WriteByte('\n')
			current.Reset()
			currentWidth = 0
		}
		for _, word := range strings.Fields(line) {
			wordWidth := runewidth.StringWidth(word)
			if currentWidth > 0 && currentWidth+1+wordWidth <= available {
				current.WriteByte(' ')
				currentWidth++
			} else if currentWidth > 0 {
				writeCurrent()
				wrapped.WriteString(continuation)
			}
			for wordWidth > available {
				fragment, rest := splitTerminalWord(word, available)
				current.WriteString(fragment)
				writeCurrent()
				wrapped.WriteString(continuation)
				word = rest
				wordWidth = runewidth.StringWidth(word)
			}
			current.WriteString(word)
			currentWidth += wordWidth
		}
		if currentWidth > 0 {
			writeCurrent()
		}
	}
	return strings.TrimSuffix(wrapped.String(), "\n")
}

func splitTerminalWord(word string, width int) (string, string) {
	var head strings.Builder
	used := 0
	runes := []rune(word)
	for i, r := range runes {
		runeWidth := runewidth.RuneWidth(r)
		if used+runeWidth > width {
			return head.String(), string(runes[i:])
		}
		head.WriteRune(r)
		used += runeWidth
	}
	return head.String(), ""
}

func (m *tuiModel) toggleCurrentItem() {
	items := m.filteredItems()
	if len(items) == 0 || m.cursor >= len(items) {
		return
	}
	item := items[m.cursor]
	if item.Status != StatusAvailable {
		m.message = "This item cannot be moved: " + item.Reason
		return
	}
	m.selectedItems[item.ID] = !m.selectedItems[item.ID]
	if !m.selectedItems[item.ID] {
		delete(m.selectedItems, item.ID)
	}
	m.message = ""
}

func (m tuiModel) filteredItems() []Item {
	if m.search == "" {
		return m.items
	}
	query := strings.ToLower(m.search)
	var result []Item
	for _, item := range m.items {
		if strings.Contains(strings.ToLower(item.Name), query) || strings.Contains(strings.ToLower(item.SourcePath), query) || strings.Contains(strings.ToLower(string(item.Category)), query) {
			result = append(result, item)
		}
	}
	return result
}

func (m tuiModel) selectedItemList() []Item {
	var result []Item
	for _, item := range m.items {
		if m.selectedItems[item.ID] && item.Status == StatusAvailable {
			result = append(result, item)
		}
	}
	return result
}

func (m tuiModel) restorableOperations() []Operation {
	var result []Operation
	for _, op := range m.operations {
		if op.Phase == "completed" || op.Phase == "cleanup_pending" {
			result = append(result, op)
		}
	}
	return result
}

func completedTransferStats(operations []Operation) (int, int64) {
	count := 0
	var bytes int64
	for _, op := range operations {
		if op.Phase != "completed" {
			continue
		}
		count++
		bytes += op.SizeBytes
	}
	return count, bytes
}

func (m tuiModel) selectedOperationList() []Operation {
	var result []Operation
	for _, op := range m.restorableOperations() {
		if m.selectedOps[op.ID] {
			result = append(result, op)
		}
	}
	return result
}

func (m *tuiModel) updateRestoreConfirm(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	return m, nil
}

func visibleRange(cursor, count, limit int) (int, int) {
	if count <= limit {
		return 0, count
	}
	start := cursor - limit/2
	if start < 0 {
		start = 0
	}
	if start+limit > count {
		start = count - limit
	}
	return start, start + limit
}

func eligibleVolumeCount(volumes []Volume) int {
	count := 0
	for _, volume := range volumes {
		if volume.Rating != RatingUnavailable {
			count++
		}
	}
	return count
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func nonempty(value, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}
