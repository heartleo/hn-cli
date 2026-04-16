Output docs/demo-comments.gif

Set Theme "Dracula"
Set Width 1920
Set Height 1080
Set FontSize 16
Set Framerate 60
Set WindowBar Colorful
Set Padding 20
Set PlaybackSpeed 1

Sleep 500ms
Type "hn best"
Sleep 500ms
Enter

# Wait for stories to load
Sleep 4s

# Select the 4th story
Down
Sleep 500ms
Down
Sleep 500ms
Down
Sleep 500ms
Enter

# Wait for comments to load
Sleep 5s

# Navigate comments with j/k
Down
Sleep 1s
Down
Sleep 1s
Down
Sleep 1s
Down
Sleep 1s
Down
Sleep 1s
Down
Sleep 1s
Down
Sleep 1s

# Fold selected comment
Space
Sleep 1s

# Navigate past folded
Down
Sleep 1s
Down
Sleep 1s

# Unfold
Up
Sleep 1s
Up
Sleep 1s
Space
Sleep 1s

# Fold all comments
Type "C"
Sleep 2s

# Unfold all comments
Type "E"
Sleep 2s

# Jump to root comment
Type "r"
Sleep 1s

# Back to top
Type "gg"
Sleep 1s

# Back to list
Escape
Sleep 2s
