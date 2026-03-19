# Copyright RetailNext, Inc. 2026

schema_version = 1

project {
  copyright_year = 2026
  copyright_holder = "RetailNext, Inc."

  header_ignore = [
    # examples used within documentation (prose)
    "examples/**",

    # GitHub issue template configuration
    ".github/ISSUE_TEMPLATE/*.yml",

    # golangci-lint tooling configuration
    ".golangci.yml",

    # GoReleaser tooling configuration
    ".goreleaser.yml",

    # testdata
    "**/testdata/**"
  ]
}
