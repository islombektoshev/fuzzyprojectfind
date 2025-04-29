build:
	go build -o fuzzyfind main.go

install:
	cp fuzzyfind ~/dotfiles | true
	cp ff.sh ~/dotfiles | true

install-zsh:
	echo "source dotfiles/ff.sh" >> ~/.zshrc
