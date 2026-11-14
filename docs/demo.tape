Output docs/demo.gif

Require git
Require go

Set Shell "bash"
Set FontSize 15
Set Width 1100
Set Height 620
Set Padding 24
Set Theme "GitHub Dark"
Set TypingSpeed 45ms
Set WindowBar Colorful

Hide
Type `export PS1='> ' PATH="$PWD/dist/demo:$PATH" && go build -o dist/demo/cmt ./cmd/cmt && cd "$(mktemp -d)" && git init -q -b main && git config user.name demo && git config user.email demo@example.com && git commit -q --allow-empty -m "chore: initial commit"`
Enter
Type `mkdir -p cli docs && printf 'package cli\n\nfunc Greet(name string) string {\n\treturn "Hello, " + name\n}\n' > cli/greet.go && clear`
Enter
Sleep 1s
Show

Type "git add cli/greet.go"
Enter
Sleep 500ms
Type "cmt"
Enter
Wait+Line@60s /cancel:/
Sleep 2s
Type "r"
Enter
Wait+Line@60s /cancel:/
Sleep 2s
Type "y"
Enter
Wait@30s
Sleep 2s

Hide
Type `printf 'package cli\n\nimport "strings"\n\nfunc Greet(name string) string {\n\tif strings.TrimSpace(name) == "" {\n\t\tname = "world"\n\t}\n\treturn "Hello, " + name\n}\n' > cli/greet.go && clear`
Enter
Show

Type `cmt -a --dry-run -c "closes #42"`
Enter
Wait@60s
Sleep 3s

Type "cmt -a -y"
Enter
Wait@60s
Sleep 3s

Hide
Type `printf '# Usage\n\nCall Greet with a name.\n' > docs/usage.md && printf '// WIP\n' >> cli/greet.go && git add -A && clear`
Enter
Show

Type "cmt -y -- docs/"
Enter
Wait@60s
Sleep 2s

Type "git log --oneline"
Enter
Wait
Sleep 4s
