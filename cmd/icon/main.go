package main

import (
	"image"
	"image/color"
	"image/png"
	"os"
)

const canvasSize = 1024

type point struct{ x, y int }

func main() {
	icon := image.NewNRGBA(image.Rect(0, 0, canvasSize, canvasSize))
	paintBackground(icon)
	paintGlow(icon, 780, 210, 340, color.NRGBA{R: 46, G: 215, B: 181, A: 70})
	paintGlow(icon, 220, 850, 300, color.NRGBA{R: 33, G: 105, B: 112, A: 80})

	outerShield := []point{{260, 214}, {764, 214}, {732, 642}, {512, 830}, {292, 642}}
	innerShield := []point{{287, 246}, {737, 246}, {704, 624}, {512, 790}, {320, 624}}
	fillPolygon(icon, outerShield, color.NRGBA{R: 194, G: 255, B: 236, A: 255})
	fillPolygon(icon, innerShield, color.NRGBA{R: 14, G: 47, B: 48, A: 255})

	mark := color.NRGBA{R: 235, G: 255, B: 249, A: 255}
	fillRect(icon, 382, 330, 444, 680, mark)
	fillRect(icon, 430, 330, 610, 392, mark)
	fillRect(icon, 555, 370, 617, 506, mark)
	fillRect(icon, 430, 475, 610, 537, mark)
	fillPolygon(icon, []point{{496, 518}, {568, 500}, {663, 680}, {586, 680}}, mark)

	file, err := os.Create("build/appicon.png")
	if err != nil {
		panic(err)
	}
	defer file.Close()
	if err := png.Encode(file, icon); err != nil {
		panic(err)
	}
}

func paintBackground(icon *image.NRGBA) {
	for y := 0; y < canvasSize; y++ {
		for x := 0; x < canvasSize; x++ {
			if !insideRoundedSquare(x, y, 72) {
				continue
			}
			mix := float64(y) / float64(canvasSize-1)
			icon.SetNRGBA(x, y, color.NRGBA{
				R: uint8(12 + 7*mix),
				G: uint8(42 + 28*mix),
				B: uint8(43 + 27*mix),
				A: 255,
			})
		}
	}
}

func insideRoundedSquare(x, y, radius int) bool {
	nearestX := clamp(x, radius, canvasSize-1-radius)
	nearestY := clamp(y, radius, canvasSize-1-radius)
	dx, dy := x-nearestX, y-nearestY
	return dx*dx+dy*dy <= radius*radius
}

func paintGlow(icon *image.NRGBA, centerX, centerY, radius int, glow color.NRGBA) {
	for y := max(0, centerY-radius); y < min(canvasSize, centerY+radius); y++ {
		for x := max(0, centerX-radius); x < min(canvasSize, centerX+radius); x++ {
			dx, dy := x-centerX, y-centerY
			distanceSquared := dx*dx + dy*dy
			if distanceSquared >= radius*radius {
				continue
			}
			strength := 1 - float64(distanceSquared)/float64(radius*radius)
			over := glow
			over.A = uint8(float64(over.A) * strength)
			blend(icon, x, y, over)
		}
	}
}

func fillRect(icon *image.NRGBA, left, top, right, bottom int, fill color.NRGBA) {
	for y := top; y < bottom; y++ {
		for x := left; x < right; x++ {
			icon.SetNRGBA(x, y, fill)
		}
	}
}

func fillPolygon(icon *image.NRGBA, polygon []point, fill color.NRGBA) {
	minimumX, maximumX := polygon[0].x, polygon[0].x
	minimumY, maximumY := polygon[0].y, polygon[0].y
	for _, vertex := range polygon[1:] {
		minimumX, maximumX = min(minimumX, vertex.x), max(maximumX, vertex.x)
		minimumY, maximumY = min(minimumY, vertex.y), max(maximumY, vertex.y)
	}
	for y := minimumY; y <= maximumY; y++ {
		for x := minimumX; x <= maximumX; x++ {
			if pointInPolygon(x, y, polygon) {
				icon.SetNRGBA(x, y, fill)
			}
		}
	}
}

func pointInPolygon(x, y int, polygon []point) bool {
	inside := false
	previous := len(polygon) - 1
	for current := range polygon {
		a, b := polygon[current], polygon[previous]
		crosses := (a.y > y) != (b.y > y)
		if crosses && x < (b.x-a.x)*(y-a.y)/(b.y-a.y)+a.x {
			inside = !inside
		}
		previous = current
	}
	return inside
}

func blend(icon *image.NRGBA, x, y int, over color.NRGBA) {
	under := icon.NRGBAAt(x, y)
	alpha := float64(over.A) / 255
	icon.SetNRGBA(x, y, color.NRGBA{
		R: uint8(float64(over.R)*alpha + float64(under.R)*(1-alpha)),
		G: uint8(float64(over.G)*alpha + float64(under.G)*(1-alpha)),
		B: uint8(float64(over.B)*alpha + float64(under.B)*(1-alpha)),
		A: under.A,
	})
}

func clamp(value, minimum, maximum int) int {
	return min(max(value, minimum), maximum)
}
