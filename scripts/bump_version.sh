#!/usr/bin/env bash

set -e

NEW_VERSION=$(git describe --tags --abbrev=0 | awk -F. '{OFS="."; $NF+=1; print $0}')

echo Creating $NEW_VERSION...
git tag -m "release ${NEW_VERSION}" -a "$NEW_VERSION"

echo 
read -p "Do you want to release the new version? [y/N] " -n 1 -r
echo    # (optional) move to a new line
if [[ ! $REPLY =~ ^[Yy]$ ]]
then
    echo "Exited without releasing, make sure you push the tag to release"
    exit 1
fi

git push origin $NEW_VERSION