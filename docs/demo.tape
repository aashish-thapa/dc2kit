# README demo. Render from the repo root with: vhs docs/demo.tape
# Needs vhs (https://github.com/charmbracelet/vhs) and dc2kit on PATH.
Output docs/demo.gif

Set Shell bash
Set FontSize 16
Set Width 1440
Set Height 680
Set Padding 24
Set Theme "Catppuccin Mocha"
Set TypingSpeed 45ms

Hide
Type "export PS1='$ ' && rm -rf /tmp/dc2kit-demo && cp -R docs/demo /tmp/dc2kit-demo && cd /tmp/dc2kit-demo && clear"
Enter
Show

Type "cat .devcontainer/devcontainer.json"
Enter
Sleep 4s

Type "clear && dc2kit ."
Enter
Sleep 10s

Type "clear && sed -n '/lifecycle@1/,/container:/p' sbx-kit/web-app.yaml"
Enter
Sleep 6s
