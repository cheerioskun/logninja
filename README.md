# LogNinja

A terminal-based log bundle refinement tool that turns massive log collections into focused, actionable datasets.

## Purpose

LogNinja solves the problem of overwhelming log bundles. When you have gigabytes of logs scattered across hundreds of files, finding what matters becomes impossible. LogNinja provides an interactive interface to slice through the noise—select specific files, filter by time ranges, apply regex patterns, and visualize log volume—all before committing to expensive operations.

Instead of blindly copying entire log directories or writing complex scripts, you refine first, then export exactly what you need.

## Core Capabilities ( IN PROGRESS )

**Massive Scale Efficiency**: Handle 30-40GB log bundles without breaking a sweat. Content-based log detection, intelligent sampling, and streaming operations keep memory usage minimal while processing thousands of files.

**Smart Log Detection**: Automatically identifies log files through content analysis rather than file extensions. Finds logs hiding in unexpected places and ignores false positives.

**Interactive Refinement**: Visual file tree with multi-select, regex include/exclude patterns, time range filtering, and real-time volume histograms.

**Time-Aware Processing**: Extract timestamp patterns from logs to enable precise temporal filtering. Visualize log volume over time and select specific time windows.

## Usage

```bash
logninja tui /path/to/log/bundle
```

Navigate with vim-style keys, select files with space, filter with regex patterns, adjust time ranges, and export refined bundles. The TUI shows real-time size estimates and volume distribution as you refine your selection.

---

Built with Go and the Charm TUI framework for responsive performance on large datasets.
