## Brief overview

This style guide defines the coding standards, organization patterns, and commenting philosophy for the PorTTY project. These guidelines ensure consistency across Go files, shell scripts, HTML, CSS, and JavaScript files, emphasizing minimal commenting, clear organization, and self-documenting code.

## File organization structure

- Use consistent section dividers:
  - `// ============================================================================` for Go
  - `# ============================================================================` for shell
  - `/* ============================================================================ */` for CSS
  - `// ============================================================================` for JavaScript
- Organize all Go files with the same structure:
  1. Package declaration and imports
  2. Constants and type definitions
  3. Global variables and configuration
  4. Utility functions
  5. Core business logic
  6. Main execution logic
- Organize frontend files with consistent structure (see Frontend-specific guidelines)
- Apply section headers with descriptive comments in ALL CAPS
- Group related functions under appropriate sections

## Minimal commenting philosophy

- Use function-level documentation only (Go doc comments for exported functions, brief comments for unexported)
- Avoid inline comments unless explaining complex logic or non-obvious behavior
- Make code self-documenting through clear naming conventions
- Remove explanatory comments that restate what the code does
- Example: Remove `// Parse the address` before `host, port, err := parseAddress(address)`

## Naming conventions

- **Go**: Follow standard Go conventions:
  - `PascalCase` for exported functions, types, and constants
  - `camelCase` for unexported functions and variables
  - `UPPER_CASE` for package-level constants (when appropriate)
  - Interface names should end with `-er` when possible
- **Shell**: `UPPER_CASE` for script-level variables, `snake_case` for function names
- **CSS**:
  - `kebab-case` for class names and IDs
  - `--kebab-case` for CSS custom properties (variables)
  - `UPPER_CASE` for CSS constants in comments
- **JavaScript**:
  - `camelCase` for functions, variables, and methods
  - `PascalCase` for classes and constructors
  - `UPPER_CASE` for constants
- **HTML**:
  - `kebab-case` for attributes and IDs
  - Semantic element names over generic divs
- Use descriptive names that eliminate need for comments
- Functions should clearly indicate their purpose through naming

## Function documentation

- **Go**: Use standard Go doc comments for exported functions with brief purpose description
- **Shell**: Add single-line comment describing function purpose
- **JavaScript**: Use JSDoc-style comments for classes and complex functions only
- **CSS**: Document complex selectors or non-obvious styling decisions
- **HTML**: Use comments sparingly, only for major sections
- Format: `// Brief description of function purpose` or `# Brief description of function purpose`
- Keep documentation concise and focused on what, not how

## Visual consistency

- Apply decorative section dividers to all files for visual organization
- Use consistent spacing and indentation:
  - Go: Use `gofmt` standard formatting (tabs for indentation)
  - Shell: 4 spaces for indentation
  - CSS: 4 spaces for indentation, consistent property ordering
  - JavaScript: 4 spaces for indentation, consistent formatting
  - HTML: 4 spaces for indentation, proper nesting
- Maintain uniform logging patterns across the project
- Keep consistent error message formats
- Use consistent font family: JetBrains Mono across all UI elements

## Error handling patterns

- Use Go's idiomatic error handling with consistent patterns
- Use consistent logging functions: `log.Printf()`, `log.Fatalf()` for different severity levels
- Apply same exit codes and error propagation patterns
- Maintain uniform error message formatting across Go and shell scripts
- Wrap errors with context using `fmt.Errorf()` with `%w` verb

## Go-specific guidelines

- **Package Structure**: Follow Go conventions with clear package boundaries
- **Interfaces**: Keep interfaces small and focused (interface segregation)
- **Concurrency**: Use channels and goroutines idiomatically, with proper cleanup
- **Context**: Use `context.Context` for cancellation and timeouts
- **Testing**: Follow Go testing conventions with `_test.go` files

## Project-specific patterns

- **WebSocket Handling**: Maintain consistent connection lifecycle management
- **PTY Management**: Use consistent patterns for terminal session handling
- **Configuration**: Use constants for default values and configuration
- **Logging**: Maintain consistent log levels and message formats across components

## Code maintenance approach

- Prioritize readability over brevity
- Ensure changes maintain consistency across all project files
- Apply style changes systematically to all relevant files
- Review for consistency when adding new functions or sections
- Use `go fmt`, `go vet`, and `golint` tools to maintain code quality
- Follow semantic versioning for releases

## Frontend-specific guidelines

### HTML Structure
- Use semantic HTML5 elements (`<main>`, `<section>`, `<article>`, etc.)
- Organize with consistent structure:
  1. DOCTYPE and meta tags
  2. External resource links (CSS, fonts)
  3. Body content with semantic structure
  4. Script tags at end of body
- Use descriptive IDs and classes that reflect purpose, not appearance
- Include proper PWA meta tags and manifest references

### CSS Organization
- Use CSS custom properties for centralized configuration:
  ```css
  :root {
      --font-family: 'JetBrains Mono', monospace;
      --font-size: 14px;
      --background-color: #000000;
  }
  ```
- Organize stylesheets with consistent structure:
  1. CSS custom properties (variables)
  2. Base styles (html, body)
  3. Layout components
  4. UI components
  5. Responsive adjustments
  6. Animations and transitions
- Use CSS custom properties instead of hardcoded values
- Group related styles under section comments
- Prefer CSS Grid and Flexbox for layouts
- Use consistent naming for component classes

### JavaScript Organization
- Use ES6+ features consistently (const/let, arrow functions, classes)
- Organize JavaScript files with consistent structure:
  1. Constants and configuration
  2. Utility functions
  3. Class definitions
  4. Main initialization logic
  5. Event listeners and handlers
- Use classes for complex functionality (e.g., ConnectionStatusManager)
- Prefer `const` over `let`, avoid `var`
- Use template literals for string interpolation
- Handle errors gracefully with try/catch blocks
- Use meaningful variable names that describe data, not types

### PWA Standards
- Include proper service worker registration
- Use cache-first strategy for static assets
- Include comprehensive manifest.json
- Handle offline states gracefully
- Use proper PWA meta tags and icons

## Shell script guidelines

- Use `#!/usr/bin/env bash` shebang for portability
- Set `set -e` for error handling
- Use consistent variable naming and quoting
- Maintain the same sectioning and commenting approach as Go files