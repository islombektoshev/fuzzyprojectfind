#!/bin/zsh

function ff() {
  target=$(~/dotfiles/fuzzyfind "$@")

  if [ -d "$target" ]; then
    cd "$target"
  else
    echo "Not a directory:\n $target"
  fi
}

