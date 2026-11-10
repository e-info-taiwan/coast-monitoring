# Reef Check reference templates

These embedded workbooks are derived from the user-supplied reference files:

- `line.xlsx`: https://docs.google.com/spreadsheets/d/1lah1eJWKiw9DI_hMUvmttPY27iFoyysj/edit
- `belt.xlsx`: https://docs.google.com/spreadsheets/d/1I_u7VwFPLvt6ZN9l7__QSd4f_H8IWKy4/edit
- `summary.xlsx`: https://docs.google.com/spreadsheets/d/19iBqIiOdASfkjrXTPxwIngc2mbWN6VtP/edit

Generate them from the downloaded original files using:

```sh
python3 scripts/prepare-report-templates.py line-original.xlsx belt-original.xlsx summary-original.xlsx
```

The developer-only script requires openpyxl for translating shared formulas. It does **not** save workbooks through openpyxl; it patches OOXML parts, preserving styles, drawings and print settings. The Go application has no Python or additional runtime dependencies.

The prepared templates contain labels, formula definitions and empty input cells. Example survey inputs, formula/chart value caches and unused shared strings are removed. Summary scratch sheets and dedicated mud-underlayer charts are excluded. At export time the Go renderer populates the inputs, guarded calculations and chart caches, and replaces the original sample titles.

Template updates must pass the layout, sample-data removal, missing-value and chart-reference tests in `internal/report/template_test.go`.


## Yeliu field reports

The standard Line, Belt and Site downloads use embedded XLSX forms derived from the three supplied Yeliu workbooks. Their DATA grids, field sheets, merges and cell styles follow the July 2025 forms. Chart series are rebuilt from the exported data. Historical Excel form controls are represented as printable choices.

- [Line](https://docs.google.com/spreadsheets/d/1xG-6d9ShiN_bWnMG_ls8YaUsg7NCsnZv/edit)
- [Belt](https://docs.google.com/spreadsheets/d/1wgyE9fjXQHbFjav_2T23iFowvc9cgRBF/edit)
- [Site](https://docs.google.com/spreadsheets/d/1RLDKJI0HaQHZkgf7Lv8IAHiRDNqzKIvX/edit)

To regenerate, install xlrd 2.0.2 and XlsxWriter 3.3.1, then run:

```sh
python3 scripts/prepare-field-templates.py original-line.xls original-belt.xls original-site.xls
```

The preparation script verifies the original files before copying static labels and layout. Keep the populated original surveys outside version control.
