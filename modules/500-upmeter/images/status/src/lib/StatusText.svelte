<script lang="ts">
	export let text = "";
	export let status = "";
	export let mute = false;

	function mutedOrStatusTextClassName(mute, status) {
		if (mute) {
			return "text-muted";
		}
		return statusTextClassName(status);
	}

	function statusTextClassName(status) {
		switch (status) {
			case "Operational":
				return "text-success";
			case "Degraded":
				return "text-warning";
			case "Outage":
				return "text-danger";
			default:
				return "";
		}
	}
</script>

<p class={mutedOrStatusTextClassName(mute, status)}>{text}</p>
