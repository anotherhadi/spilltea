package teapot

import "strings"

// FrameLines returns the number of visual lines in a teapot frame.
func FrameLines() int {
	frames := TeapotFrames()
	if len(frames) == 0 {
		return 0
	}
	return strings.Count(frames[0], "\n") + 1
}

func Teapot() string {
	return "" +
		"             )  \n" +
		"            (   \n" +
		"             )  \n" +
		" .-.,--^--.  _  \n" +
		" \\\\| `---' |//\n" +
		"  \\|        /  \n" +
		"  _\\_______/_  "
}

func TeapotFrames() []string {
	return []string{
		"" +
			"             )  \n" +
			"            (   \n" +
			"             )  \n" +
			" .-.,--^--.  _  \n" +
			" \\\\| `---' |//\n" +
			"  \\|        /  \n" +
			"  _\\_______/_  ",

		"" +
			"            )   \n" +
			"           (    \n" +
			"            )   \n" +
			" .-.,--^--.  _  \n" +
			" \\\\| `---' |//\n" +
			"  \\|        /  \n" +
			"  _\\_______/_  ",

		"" +
			"              ) \n" +
			"             (  \n" +
			"              ) \n" +
			" .-.,--^--.  _  \n" +
			" \\\\| `---' |//\n" +
			"  \\|        /  \n" +
			"  _\\_______/_  ",

		"" +
			"                \n" +
			"             (  \n" +
			"              ) \n" +
			" .-.,--^--.  _  \n" +
			" \\\\| `---' |//\n" +
			"  \\|        /  \n" +
			"  _\\_______/_  ",

		"" +
			"                \n" +
			"             (( \n" +
			"              ) \n" +
			" .-.,--^--.  _  \n" +
			" \\\\| `---' |//\n" +
			"  \\|        /  \n" +
			"  _\\_______/_  ",
	}
}
