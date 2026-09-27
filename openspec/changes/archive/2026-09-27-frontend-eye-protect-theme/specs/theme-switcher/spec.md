## Purpose

Lets users switch the chaos-ui frontend between a terminal dark theme and a light-yellow eye-protection theme, with the choice persisted across sessions.

## ADDED Requirements

### Requirement: Theme switching
The system SHALL provide at least two themes: the existing terminal dark theme and a light-yellow eye-protection theme. The user SHALL be able to switch between them from a visible control in the page header.

#### Scenario: Switch to eye-protection theme
- **WHEN** the user clicks the theme toggle in the page header while the terminal dark theme is active
- **THEN** the system applies the light-yellow eye-protection theme immediately without a page reload

#### Scenario: Switch back to terminal dark theme
- **WHEN** the user clicks the theme toggle while the eye-protection theme is active
- **THEN** the system applies the terminal dark theme immediately without a page reload

### Requirement: Theme persistence
The system SHALL remember the user's selected theme in browser local storage and apply it on the next page load.

#### Scenario: Theme survives reload
- **WHEN** the user selects the light-yellow eye-protection theme and reloads the page
- **THEN** the eye-protection theme is applied on load

#### Scenario: First visit uses default theme
- **WHEN** a user visits the frontend for the first time with no stored theme
- **THEN** the terminal dark theme is applied

### Requirement: Eye-protection theme colors
The light-yellow eye-protection theme SHALL use a warm, low-contrast light palette: light-yellow background, dark brown/charcoal text, and desaturated accent colors. All text, borders, panels, dialogs, and terminal-styled components SHALL remain readable in this theme.

#### Scenario: Terminal components remain readable
- **WHEN** the eye-protection theme is active and the terminal frame, command palette, and dashboards are displayed
- **THEN** all text is legible against the light-yellow background with sufficient contrast, and no component is left unreadable

#### Scenario: Element Plus components follow the theme
- **WHEN** the eye-protection theme is active
- **THEN** Element Plus components (buttons, tables, dialogs, menus, inputs) use colors consistent with the light-yellow palette
