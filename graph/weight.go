package graph

const(
	PRIORITY_TIME=iota
	PRIORITY_BUDGET
)
type SimpleWeight struct{
	w uint32
}
func (this SimpleWeight) SetWeight(weight uint32){
	this.w = weight
}
func (this SimpleWeight) GetWeight(weight uint32) uint32{
	return this.w
}

func (l SimpleWeight) Compare(r SimpleWeight) bool {
	if l.w <= r.w{
		return true
	} else {
		return false
	}
}



/*
FIXME: The priority of configuration is based only on the cmpflag
of the object calling the compare function. This configuration must
be used with care.
 */

type SimpleBaseWeight struct{
	timeInMin uint32
	budget    float64
	/*
	Need to solve cmpflag match problems, make sure cmpflag matches before calling the comparison function
	 */
	cmpflag uint8
}
func (v SimpleBaseWeight) Setcmpflag(cmpflag uint8){
	/*
	Need to perform validity check of input values
	will change function signiture to bool after then
	 */
	v.cmpflag = cmpflag
}
func (v SimpleBaseWeight) Getcmpflag() uint8{
	return v.cmpflag
}

/*
FIXME: The priority of configuration is based only on the cmpflag
of the object calling the compare function. This configuration must
be used with care.
 */
func (l SimpleBaseWeight) Compare(r SimpleBaseWeight) bool{
	switch l.cmpflag {
	case PRIORITY_TIME:
		if l.timeInMin <= r.timeInMin {
			return true
		} else {
			return false
		}
	case PRIORITY_BUDGET:
		if l.budget <= r.budget {
			return true
		} else {
			return false
		}
	default:
		/*
		Default behavior favor money
		 */
		if l.budget <= r.budget {
			return true
		} else {
			return false
		}
	}
}

