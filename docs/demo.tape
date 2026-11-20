# README demo. Render from the repo root with: vhs docs/demo.tape
# Needs vhs (https://github.com/charmbracelet/vhs) and dc2kit on PATH.
Output docs/demo.gif

Set Shell bash
Set FontSize 16
Set Width 1200
Set Height 720
Set Theme "Catppuccin Mocha"
Set TypingSpeed 40ms

Hide
Type "export PS1='$ ' && rm -rf /tmp/dc2kit-demo && cp -R testdata/e2e/node-app /tmp/dc2kit-demo && cd /tmp/dc2kit-demo && clear"
Enter
Show

Type "cat .devcontainer/devcontainer.json | head -12"
Enter
Sleep 3s

Type "dc2kit ."
Enter
Sleep 8s

Type "head -30 sbx-kit/node-app.yaml"
Enter
Sleep 5s
