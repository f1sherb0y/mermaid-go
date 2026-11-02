package fcose

import "math"

// layout-base's IGeometry, on nodes' rects.

// separation is calcSeparationAmount: half of what moves a and b (which
// overlap) apart, plus buffer, signed for a.
func separation(a, b *lnode, buffer float64) (float64, float64) {
	dirX, dirY := 1.0, 1.0
	if a.centerX() < b.centerX() {
		dirX = -1
	}
	if a.centerY() < b.centerY() {
		dirY = -1
	}
	ox := math.Min(a.right(), b.right()) - math.Max(a.x, b.x)
	oy := math.Min(a.bottom(), b.bottom()) - math.Max(a.y, b.y)
	if a.x <= b.x && a.right() >= b.right() {
		ox += math.Min(b.x-a.x, a.right()-b.right())
	} else if b.x <= a.x && b.right() >= a.right() {
		ox += math.Min(a.x-b.x, b.right()-a.right())
	}
	if a.y <= b.y && a.bottom() >= b.bottom() {
		oy += math.Min(b.y-a.y, a.bottom()-b.bottom())
	} else if b.y <= a.y && b.bottom() >= a.bottom() {
		oy += math.Min(a.y-b.y, b.bottom()-a.bottom())
	}
	slope := math.Abs((b.centerY() - a.centerY()) / (b.centerX() - a.centerX()))
	if b.centerY() == a.centerY() && b.centerX() == a.centerX() {
		slope = 1.0
	}
	moveY := slope * ox
	moveX := oy / slope
	if ox < moveX {
		moveX = ox
	} else {
		moveY = oy
	}
	return -1 * dirX * (moveX/2 + buffer), -1 * dirY * (moveY/2 + buffer)
}

// intersection is getIntersection2: where the line between the centers
// leaves a (res[0], res[1]) and b (res[2], res[3]); true if they overlap.
func intersection(a, b *lnode, res *[4]float64) bool {
	p1x, p1y := a.centerX(), a.centerY()
	p2x, p2y := b.centerX(), b.centerY()
	if intersects(a, b) {
		*res = [4]float64{p1x, p1y, p2x, p2y}
		return true
	}
	topLeftAx, topLeftAy := a.x, a.y
	topRightAx := a.right()
	bottomLeftAx, bottomLeftAy := a.x, a.bottom()
	bottomRightAx := a.right()
	halfWidthA, halfHeightA := a.w/2, a.h/2
	topLeftBx, topLeftBy := b.x, b.y
	topRightBx := b.right()
	bottomLeftBx, bottomLeftBy := b.x, b.bottom()
	bottomRightBx := b.right()
	halfWidthB, halfHeightB := b.w/2, b.h/2

	clipA, clipB := false, false
	if p1x == p2x {
		if p1y > p2y {
			*res = [4]float64{p1x, topLeftAy, p2x, bottomLeftBy}
			return false
		} else if p1y < p2y {
			*res = [4]float64{p1x, bottomLeftAy, p2x, topLeftBy}
			return false
		}
		return false
	}
	if p1y == p2y {
		if p1x > p2x {
			*res = [4]float64{topLeftAx, p1y, topRightBx, p2y}
			return false
		} else if p1x < p2x {
			*res = [4]float64{topRightAx, p1y, topLeftBx, p2y}
			return false
		}
		return false
	}
	slopeA := a.h / a.w
	slopeB := b.h / b.w
	slopePrime := (p2y - p1y) / (p2x - p1x)
	if -slopeA == slopePrime {
		if p1x > p2x {
			res[0], res[1] = bottomLeftAx, bottomLeftAy
		} else {
			res[0], res[1] = topRightAx, topLeftAy
		}
		clipA = true
	} else if slopeA == slopePrime {
		if p1x > p2x {
			res[0], res[1] = topLeftAx, topLeftAy
		} else {
			res[0], res[1] = bottomRightAx, bottomLeftAy
		}
		clipA = true
	}
	if -slopeB == slopePrime {
		if p2x > p1x {
			res[2], res[3] = bottomLeftBx, bottomLeftBy
		} else {
			res[2], res[3] = topRightBx, topLeftBy
		}
		clipB = true
	} else if slopeB == slopePrime {
		if p2x > p1x {
			res[2], res[3] = topLeftBx, topLeftBy
		} else {
			res[2], res[3] = bottomRightBx, bottomLeftBy
		}
		clipB = true
	}
	if clipA && clipB {
		return false
	}
	var dirA, dirB int
	if p1x > p2x {
		if p1y > p2y {
			dirA = cardinal(slopeA, slopePrime, 4)
			dirB = cardinal(slopeB, slopePrime, 2)
		} else {
			dirA = cardinal(-slopeA, slopePrime, 3)
			dirB = cardinal(-slopeB, slopePrime, 1)
		}
	} else {
		if p1y > p2y {
			dirA = cardinal(-slopeA, slopePrime, 1)
			dirB = cardinal(-slopeB, slopePrime, 3)
		} else {
			dirA = cardinal(slopeA, slopePrime, 2)
			dirB = cardinal(slopeB, slopePrime, 4)
		}
	}
	if !clipA {
		switch dirA {
		case 1:
			res[0], res[1] = p1x+(-halfHeightA)/slopePrime, topLeftAy
		case 2:
			res[0], res[1] = bottomRightAx, p1y+halfWidthA*slopePrime
		case 3:
			res[0], res[1] = p1x+halfHeightA/slopePrime, bottomLeftAy
		case 4:
			res[0], res[1] = bottomLeftAx, p1y+(-halfWidthA)*slopePrime
		}
	}
	if !clipB {
		switch dirB {
		case 1:
			res[2], res[3] = p2x+(-halfHeightB)/slopePrime, topLeftBy
		case 2:
			res[2], res[3] = bottomRightBx, p2y+halfWidthB*slopePrime
		case 3:
			res[2], res[3] = p2x+halfHeightB/slopePrime, bottomLeftBy
		case 4:
			res[2], res[3] = bottomLeftBx, p2y+(-halfWidthB)*slopePrime
		}
	}
	return false
}

func cardinal(slope, slopePrime float64, line int) int {
	if slope > slopePrime {
		return line
	}
	return 1 + line%4
}
