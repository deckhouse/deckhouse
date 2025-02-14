<script lang="ts">
	export let text = "";
	export let status = "";

	function statusTextClassName(status) {
		switch (status) {
			case "Operational":
				return "text-success";
			case "Degraded":
				return "text-warning";
			case "Outage":
				return "text-danger";
			default:
				return "text-muted";
		}
	}
</script>

<p class={`${statusTextClassName(status)} m-0`}>{text || "No data for 15 min"}</p>
