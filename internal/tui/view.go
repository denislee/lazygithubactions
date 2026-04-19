package tui

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/dns/lazygithubactions/internal/tui/components"
	"github.com/dns/lazygithubactions/internal/tui/theme"
)

func (a App) View() tea.View {
	if a.width == 0 {
		v := tea.NewView("Loading...")
		v.AltScreen = true
		return v
	}

	var content string

	switch a.ActiveView {
	case DetailView:
		a.runDetail.SetSize(a.width, a.height-2)
		content = lipgloss.JoinVertical(lipgloss.Left,
			a.runDetail.View(),
			a.statusBar.View(),
			components.HelpBar(a.width, "detail"),
		)

	case LogView:
		a.logViewer.SetSize(a.width, a.height)
		content = a.logViewer.View()

	default: // MainView and overlay views
		content = a.renderMainLayout()
	}

	v := tea.NewView(content)
	v.AltScreen = true
	return v
}

func (a App) renderMainLayout() string {
	leftWidth := max(a.width/4, 20)
	runWidth := a.width - leftWidth
	totalHeight := a.height - 2
	innerWidth := leftWidth - 4

	a.orgSelector.SetFocused(a.activePanel == orgPanel)
	a.repoList.SetFocused(a.activePanel == repoPanel)
	a.runList.SetFocused(a.activePanel == runPanel)

	// Build left panel content: org + divider + repos in a single frame.
	orgContent := a.orgSelector.ViewContent()
	divider := lipgloss.NewStyle().Foreground(lipgloss.Color("#666666")).
		Render(strings.Repeat("─", innerWidth))

	var repoContent string
	if a.loadingRepos {
		repoContent = theme.TitleStyle.Render("Repositories") + "\n"
		repoContent += "  " + a.spinner.View() + " Loading..."
	} else {
		repoContent = a.repoList.ViewContent()
	}

	leftContent := orgContent + divider + "\n" + repoContent

	focused := a.activePanel == orgPanel || a.activePanel == repoPanel
	leftStyle := theme.PanelStyle
	if focused {
		leftStyle = theme.ActivePanelStyle
	}
	leftCol := leftStyle.Width(leftWidth).Height(totalHeight).Render(leftContent)

	runView := a.runList.View()
	if a.loadingRuns {
		runView = a.renderLoadingPanel(
			fmt.Sprintf("Workflow Runs — %s", a.runList.Repo()),
			runWidth, totalHeight, a.activePanel == runPanel,
		)
	}

	panels := lipgloss.JoinHorizontal(lipgloss.Top, leftCol, runView)

	mainContent := lipgloss.JoinVertical(lipgloss.Left,
		panels,
		a.statusBar.View(),
		components.HelpBar(a.width, ""),
	)

	switch {
	case a.ActiveView == QuickSwitchView && a.quickSwitch != nil:
		return a.renderOverlay(mainContent, a.quickSwitch.View())
	case a.ActiveView == TriggerView && a.triggerDlg != nil:
		return a.renderOverlay(mainContent, a.triggerDlg.View())
	case a.ActiveView == ConfirmView && a.confirmDlg != nil:
		return a.renderOverlay(mainContent, a.confirmDlg.View())
	default:
		return mainContent
	}
}

func (a App) renderLoadingPanel(title string, width, height int, focused bool) string {
	content := theme.TitleStyle.Render(title) + "\n\n"
	content += "  " + a.spinner.View() + " Loading..."
	style := theme.PanelStyle
	if focused {
		style = theme.ActivePanelStyle
	}
	return style.Width(width).Height(height).Render(content)
}

func (a App) renderOverlay(_ string, overlay string) string {
	return lipgloss.Place(a.width, a.height, lipgloss.Center, lipgloss.Center, overlay)
}

func (a *App) layoutComponents() {
	a.statusBar.SetWidth(a.width)

	leftWidth := max(a.width/4, 20)
	runWidth := a.width - leftWidth
	totalHeight := a.height - 2

	innerWidth := leftWidth - 4
	orgContentLines := max(min(a.orgSelector.OrgCount(), 5), 1)
	orgSectionHeight := orgContentLines + 1

	repoContentHeight := max(totalHeight-2-orgSectionHeight-1, 3)

	a.orgSelector.SetSize(innerWidth, orgContentLines)
	a.repoList.SetSize(innerWidth, repoContentHeight-1)
	a.runList.SetSize(runWidth, totalHeight)
	a.runDetail.SetSize(a.width, a.height-2)
	a.logViewer.SetSize(a.width, a.height)
}
