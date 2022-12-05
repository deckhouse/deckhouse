<script lang="ts">
	import { Col } from "sveltestrap";
	import StatusText from "./StatusText.svelte";

	export let name = "Unknown";
	export let description = "Waiting to be filled...";
	export let status = "Operational";
	export let mute = false;
</script>

<Col class="col-9">
	<h4 class="font-weight-normal">{name}</h4>
	<p class="text-muted">{description}</p>
</Col>
<Col class="col-3 text-end">
	<h4 class="font-weight-light">
		<StatusText {status} text={status} {mute} />
	</h4>
</Col>
