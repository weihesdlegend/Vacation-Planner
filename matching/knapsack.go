package matching

import (
	"github.com/weihesdlegend/Vacation-planner/POI"
	"math"
)

const SelectionThreshold = -1

type knapsackNodeRecord struct {
	timeUsed uint8
	cost     uint
	score    float64
	Solution []Place
}

type knapsackRecordTable struct {
	timeLimit   uint8
	budget      uint
	SavedRecord map[uint]knapsackNodeRecord
	NewRecord   map[uint]knapsackNodeRecord
}

func (recordTable *knapsackRecordTable) Init(timeLimit uint8, budget uint) {
	recordTable.timeLimit = timeLimit
	recordTable.budget = budget
	recordTable.SavedRecord = make(map[uint]knapsackNodeRecord)
	recordTable.NewRecord = make(map[uint]knapsackNodeRecord)
	start := knapsackNodeRecord{0, 0, SelectionThreshold, make([]Place, 0)}
	recordTable.SavedRecord[recordTable.getKey(0, 0)] = start
}

func (recordTable *knapsackRecordTable) getKey(timeLimit uint8, budget uint) (key uint) {
	key = uint(timeLimit)*recordTable.budget + budget
	return
}

func (recordTable *knapsackRecordTable) getTimeLimitAndCost(key uint) (timeLimit uint8, budget uint) {
	budget = key % recordTable.budget
	timeLimit = uint8((key - budget) / recordTable.budget)
	return
}

func (recordTable *knapsackRecordTable) update() {
	for key, record := range recordTable.NewRecord {
		if oldRecord, ok := recordTable.SavedRecord[key]; ok {
			if oldRecord.score < record.score {
				recordTable.SavedRecord[key] = record
			}
		} else {
			recordTable.SavedRecord[key] = record
		}
		delete(recordTable.NewRecord, key)
	}
}

/*
	KnapsackV1 v2 uses sparse matrix like storage for step values and saves memory
	KnapsackV1 v1 is migrated to knapsack_old_test_only.go
*/
func Knapsack(places []Place, interval QueryTimeInterval, budget uint) (results []Place, totalCost uint, totalTimeSpent uint8) {
	//Initialize knapsack data structures
	var recordTable knapsackRecordTable
	timeLimit := interval.EndHour - interval.StartHour
	rt := &recordTable
	rt.Init(timeLimit, budget)
	optimalNode := knapsackNodeRecord{0, 0, SelectionThreshold, make([]Place, 0)}
	//DP process
	var stayTime POI.StayingTime
	for _, place := range places {
		rt.update()
		stayTime = POI.GetStayingTimeForLocationType(place.GetPlaceType())
		for key, record := range rt.SavedRecord {
			currentTimeSpent, curCost := rt.getTimeLimitAndCost(key)
			currentQueryStartTime, _ := interval.AddOffsetHours(currentTimeSpent)
			newTimeSpent := currentTimeSpent + uint8(stayTime)
			newCost := curCost + uint(math.Ceil(place.Price))
			if newTimeSpent <= rt.timeLimit && newCost <= budget && place.IsOpenBetween(currentQueryStartTime, uint8(stayTime)) {
				newKey := rt.getKey(newTimeSpent, newCost)
				newSolution := make([]Place, len(record.Solution))
				copy(newSolution, record.Solution)
				newSolution = append(newSolution, place)
				newScore := ScoreNoDistance(newSolution)
				newRecord := knapsackNodeRecord{newTimeSpent, newCost, newScore, newSolution}
				if alreadyRecord, ok := rt.NewRecord[newKey]; ok {
					if alreadyRecord.score < newRecord.score {
						rt.NewRecord[newKey] = newRecord
					}
				} else {
					rt.NewRecord[newKey] = newRecord
				}
				if newScore > optimalNode.score {
					optimalNode.score = newScore
					optimalNode.cost = newCost
					optimalNode.timeUsed = newTimeSpent
					optimalNode.Solution = append([]Place(nil), newSolution...)
				}
			}
		}
	}
	return optimalNode.Solution, optimalNode.cost, optimalNode.timeUsed
}
