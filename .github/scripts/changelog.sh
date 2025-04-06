#!/bin/bash
set -e

check_changelog() {
    if git diff --name-only HEAD^ HEAD | grep -q "CHANGELOG"; then
        echo "true"
    else
        echo "false"
    fi
}

get_version() {
    # Extract version from CHANGELOG
    VERSION=$(grep -m 1 "## \[" CHANGELOG.md | sed -E 's/## \[([0-9]+\.[0-9]+\.[0-9]+)\].*/\1/')
    if [[ $VERSION =~ ^[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
        echo "$VERSION"
    else
        echo "Error: Invalid version format in CHANGELOG" >&2
        exit 1
    fi
}

create_tag() {
    VERSION=$1
    DRY_RUN=$2

    git config --local user.email "action@github.com"
    git config --local user.name "GitHub Action"

    if [ "$DRY_RUN" = "true" ]; then
        echo "Would create tag: v$VERSION"
    else
        git tag -a "v$VERSION" -m "Release v$VERSION"
        git push origin "v$VERSION"
    fi
}

# Call the function based on the first argument
case "$1" in
    "check-changelog")
        check_changelog
        ;;
    "get-version")
        get_version
        ;;
    "create-tag")
        create_tag "$2" "$3"
        ;;
    *)
        echo "Unknown command: $1"
        exit 1
        ;;
esac
