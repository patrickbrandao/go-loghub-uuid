//go:build race

package tests

// raceDetectorEnabled informa se a suíte foi compilada com -race. Veja
// skipIfRaceDetector em alloc_test.go.
const raceDetectorEnabled = true
