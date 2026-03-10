# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [0.1.0] - 2026-01-18

### Added
- Initial release of `rich_rust`, a port of Python's Rich library.
- **Core:** `Console`, `Style`, `Text`, `Segment` primitives.
- **Renderables:**
  - `Table`: Auto-sizing columns, borders, headers/footers.
  - `Panel`: Boxed content with titles and subtitles.
  - `Tree`: Hierarchical data visualization with guide lines.
  - `ProgressBar`: Customizable progress tracking with spinners.
  - `Rule`: Horizontal divider lines.
  - `Columns`: Newspaper-style multi-column layout.
  - `Padding` & `Align`: Layout helpers.
- **Markup:** Rich text markup parsing (e.g., `[bold red]Text[/]`).
- **Terminal:** Automatic color system detection (TrueColor, 256-color, 16-color) and legacy Windows support.
- **Features:**
  - `syntax`: Syntax highlighting via `syntect`.
  - `markdown`: Markdown rendering via `pulldown-cmark`.
  - `json`: JSON pretty-printing with syntax highlighting.

### Changed
- **Optimization:** Refactored `Segment` to use `Cow<'a, str>` for zero-allocation rendering support.
- **Optimization:** `Text::render` now uses byte-index slicing to avoid allocating intermediate `String`s.
- **Optimization:** `Style::render_ansi` now returns `Arc` cached strings to reduce cloning in the rendering hot path.
- **Concurrency:** `Console` is `Send` but not `Sync` (intentional). Thread-safety verified via tests.

### Fixed
- Fixed `criterion::black_box` deprecation warning in benchmarks.
