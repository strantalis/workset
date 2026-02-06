package guardrails

import "testing"

func TestCountLOCGo(t *testing.T) {
	input := []byte(`package main

// line comment
/*
block
comment
*/
func main() {
	/* inline */ println("hi") // trailing
}
`)

	got := CountLOC("main.go", input)
	if got != 4 {
		t.Fatalf("CountLOC(main.go) = %d, want 4", got)
	}
}

func TestCountLOCSvelte(t *testing.T) {
	input := []byte(`<script lang="ts">
	// comment
	const x = 1;
</script>

<!-- html comment -->
<div>{x}</div>
`)

	got := CountLOC("Component.svelte", input)
	if got != 4 {
		t.Fatalf("CountLOC(Component.svelte) = %d, want 4", got)
	}
}
