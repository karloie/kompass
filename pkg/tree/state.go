package tree

type treeBuildState struct {
	visited    map[string]bool
	suppressed map[string]bool
}

func newTreeBuildState() *treeBuildState {
	return &treeBuildState{
		visited:    make(map[string]bool),
		suppressed: make(map[string]bool),
	}
}

func (s *treeBuildState) IsSeen(key string) bool {
	return s.visited[key]
}

func (s *treeBuildState) MarkSeen(key string) {
	s.visited[key] = true
}

func (s *treeBuildState) IsSuppressed(key string) bool {
	return s.suppressed[key]
}

func (s *treeBuildState) Suppress(key string) {
	s.suppressed[key] = true
}

func (s *treeBuildState) Unsuppress(key string) {
	s.suppressed[key] = false
}

func (s *treeBuildState) CanTraverse(key string) bool {
	return !s.IsSeen(key) && !s.IsSuppressed(key)
}

func markSuppressedKeys(state *treeBuildState, keys []string) {
	for _, key := range keys {
		state.Suppress(key)
	}
}

func markSuppressedSet(state *treeBuildState, keys map[string]bool) {
	for key := range keys {
		state.Suppress(key)
	}
}
