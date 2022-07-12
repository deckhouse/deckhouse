<script lang="ts">
	import { Col, Container, Row } from "sveltestrap/src";
	import Group from "./Group.svelte";
	import StatusText from "./StatusText.svelte";

	import { getGroupData } from "./en";

	async function fetchStatusJSON() {
		const r = await fetch("/public/api/status", { headers: { accept: "application/json" } });
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

<Container style="width: 720px">
	<Row class="mt-5 align-items-end">
		<Col>
			<h1 class="display-4">Status</h1>
		</Col>
		<Col class="text-right">
			<p>
				<span class="text-muted"> as of</span>
				{now.toLocaleTimeString()}
			</p>
		</Col>
	</Row>

	<hr class="mt-0" />

	{#if data == null && error == null}
		<Row class="mb-5 mt-5">
			<Col>
				<h2 class="text-muted font-weight-light">Wait a second...</h2>
			</Col>
		</Row>
	{:else if data != null}
		<h2 class="mb-5 mt-5 font-weight-normal">
			<StatusText
				status={data.status}
				text={"Cluster " + data.status + pendingUpdateText}
				mute={error != null}
			/>
		</h2>

		{#each data.rows as row}
			<Row class="mb-3">
				<Group {...getGroupData(row.group)} status={row.status} mute={error != null} />
			</Row>
		{/each}
	{/if}
</Container>
