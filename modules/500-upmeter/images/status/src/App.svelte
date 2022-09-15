<script lang="ts">
	import { Col, Container, Row, Styles } from "sveltestrap";
	import Group from "./lib/Group.svelte";
	import StatusText from "./lib/StatusText.svelte";
	import { getGroupData } from "./en";

	async function fetchStatusJSON() {
		const r = await fetch("/public/api/status", {
			headers: { accept: "application/json" },
		});
		return await r.json();
	}

	// The state
	let data = null;
	let error = null;
	let pendingUpdateText = "";
	let now = new Date();

	async function update() {
		now = new Date();
		try {
			data = await fetchStatusJSON();
			error = null;
			pendingUpdateText = "";
		} catch (e) {
			console.error(e);
			error = e;
			pendingUpdateText = " (pending update...)";
		}
	}

	// The update loop
	update();
	$: {
		setInterval(update, 10e3);
	}
</script>

<Styles />

<Container style="width: 600px">
	<Row class="mt-5 align-items-end">
		<Col>
			<h1 class="display-4">Status</h1>
		</Col>
		<Col class="text-end">
			<h2 class="display-4 fw-light">
				{#if data == null && error == null}
					Wait a second...
				{:else if data != null}
					<StatusText status={data.status} text={data.status + pendingUpdateText} mute={error != null} />
				{/if}
			</h2>
		</Col>
	</Row>

	<hr class="m-0" />

	<Row class="mb-5">
		<p class="text-end mt-2">
			<span class="text-muted"> as of</span>
			{now.toLocaleTimeString()}
		</p>
	</Row>

	{#if data != null && error == null}
		{#each data.rows as row}
			<Row class="mb-3">
				<Group {...getGroupData(row.group)} status={row.status} mute={error != null} />
			</Row>
		{/each}
	{/if}
</Container>
