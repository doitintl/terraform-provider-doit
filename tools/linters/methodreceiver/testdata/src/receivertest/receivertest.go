package receivertest

// --- BAD: mapping function as a method ---

type fooResource struct{}

func (r *fooResource) mapFooToModel(resp interface{}, state interface{}) { // want `mapping function mapFooToModel should be a free function, not a method; the receiver is not needed for data transformation`
	_ = resp
	_ = state
}

type barDataSource struct{}

func (ds *barDataSource) mapBarToModel(resp interface{}) { // want `mapping function mapBarToModel should be a free function, not a method; the receiver is not needed for data transformation`
	_ = resp
}

// --- GOOD: mapping function as a free function ---

func mapBazToModel(resp interface{}, state interface{}) {
	_ = resp
	_ = state
}

// --- GOOD: populateState is allowed as a method ---

func (r *fooResource) populateState(state interface{}) {
	_ = state
}

// --- GOOD: non-mapping methods are allowed ---

func (r *fooResource) Schema() {}

func (r *fooResource) Create() {}

func (r *fooResource) helperFunc() {}
