package matching

import (
	"github.com/weihesdlegend/Vacation-planner/POI"
	"math"
)

type knapsackNode struct {
	score    float64
	solution []Place
}

func KnapsackMatrixCopy(dst [][]knapsackNode, src [][]knapsackNode) {
	rowDst := len(dst)
	rowSrc := len(src)
	if rowDst != rowSrc || rowDst == 0 || rowSrc == 0 {
		return
	}
	colDst := len(dst[0])
	colSrc := len(src[0])
	if colDst != colSrc {
		return
	}
	for i := 0; i < rowSrc; i++ {
		for j := 0; j < colSrc; j++ {
			dst[i][j].score = src[i][j].score
			dst[i][j].solution = append(make([]Place, 0, len(src[i][j].solution)), src[i][j].solution...)
		}
	}
}

func KnapsackV1(places []Place, interval QueryTimeInterval, budget uint) (results []Place) {
	timeLimit := interval.EndHour - interval.StartHour
	//INIT KNAPSACK MATRIX
	current := make([][]knapsackNode, timeLimit+1)
	for i := 0; i < int(timeLimit)+1; i++ {
		current[i] = make([]knapsackNode, budget+1)
		for j := 0; j < int(budget)+1; j++ {
			current[i][j].score = SelectionThreshold
		}
	}
	next := make([][]knapsackNode, timeLimit+1)
	for i := 0; i < int(timeLimit)+1; i++ {
		next[i] = make([]knapsackNode, budget+1)
		for j := 0; j < int(budget)+1; j++ {
			next[i][j].score = SelectionThreshold
		}
	}
	optimalNode := knapsackNode{SelectionThreshold, make([]Place, 0)}
	tempPlaces := make([]Place, 0)
	var tempScore float64 = SelectionThreshold
	tempi := 0
	tempj := 0

	//KNAPSACK DP process
	var staytime POI.StayingTime
	for k := 0; k < len(places); k++ {
		KnapsackMatrixCopy(current, next)
		staytime = POI.GetStayingTimeForLocationType(places[k].GetPlaceType())
		//INITIALIZE 0,0
		if uint8(staytime) <= timeLimit && int(math.Ceil(places[k].Price)) <= int(budget) && places[k].IsOpenBetween(interval, uint8(staytime)) {
			tempPlaces = append(current[0][0].solution, places[k])
			tempScore = ScoreNoDistance(tempPlaces)
			if tempScore > next[staytime][int(math.Ceil(places[k].GetPrice()))].score {
				next[staytime][int(math.Round(places[k].GetPrice()))].score = tempScore
				next[staytime][int(math.Round(places[k].GetPrice()))].solution = append(make([]Place, 0, len(tempPlaces)), tempPlaces...)
			}
			if tempScore > optimalNode.score {
				optimalNode.score = tempScore
				optimalNode.solution = append(make([]Place, 0, len(tempPlaces)), tempPlaces...)
			}
		}
		//DP to the remaining matrix
		for i := 0; i < int(timeLimit); i++ {
			for j := 0; j < int(budget); j++ {
				if current[i][j].score > SelectionThreshold {
					currentQueryStart, _ := interval.AddOffsetHours(uint8(i))
					if i+int(staytime) <= int(timeLimit) && j+int(math.Ceil(places[k].Price)) <= int(budget) && places[k].IsOpenBetween(currentQueryStart, uint8(staytime)) {
						tempi = i + int(staytime)
						tempj = j + int(math.Ceil(places[k].GetPrice()))
						tempPlaces = append(current[i][j].solution, places[k])
						tempScore = ScoreNoDistance(tempPlaces)
						if tempScore > next[tempi][tempj].score {
							next[tempi][tempj].score = tempScore
							next[tempi][tempj].solution = append(make([]Place, 0, len(tempPlaces)), tempPlaces...)
						}
						if tempScore > optimalNode.score {
							optimalNode.score = tempScore
							optimalNode.solution = append(make([]Place, 0, len(tempPlaces)), tempPlaces...)
						}
					}
				}
			}
		}
	}
	return optimalNode.solution
}
