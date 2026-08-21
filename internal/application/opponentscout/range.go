package opponentscout

type RatingRangeStrategy interface {
	Bounds(rating int) (minimum int, maximum int)
}

type Fixed48RatingRangeStrategy struct{}

func (Fixed48RatingRangeStrategy) Bounds(rating int) (int, int) {
	minimum := rating - 48
	if minimum < 0 {
		minimum = 0
	}
	return minimum, rating + 48
}
