{{- define "ebpf-exporter-program-oomkill-v1" }}
#kernel-version-constraint >= 5.3.0, < 5.5.0
- name: oomkill
  metrics:
    counters:
      - name: oom_kills
        help: Count global and cgroup level OOMs
        perf_map: events
        labels:
          - name: cgroup_path
            size: 8
            decoders:
              - name: uint
              - name: cgroup
          - name: global_oom
            size: 1
            decoders:
              - name: uint
  kprobes:
    oom_kill_process: count_ooms
  code: |
    #include <uapi/linux/ptrace.h>
    #include <linux/oom.h>
    #include <linux/memcontrol.h>
    // we'll use "BPF_PERF_OUTPUT" map type here to avoid unbound cardinality
    BPF_PERF_OUTPUT(events);
    struct data_t {
        u64 cgroup_id;
        u8 global_oom;
    };
    void count_ooms(struct pt_regs *ctx, struct oom_control *oc, const char *message) {
        struct data_t data = {};
        struct mem_cgroup *mcg = oc->memcg;
        if (!mcg) {
            data.global_oom = 1;
            events.perf_submit(ctx, &data, sizeof(data));
            return;
        }
        data.cgroup_id = mcg->css.cgroup->kn->id.id;
        events.perf_submit(ctx, &data, sizeof(data));
    }
{{- end }}

{{- define "ebpf-exporter-program-oomkill-v2" }}
#kernel-version-constraint >= 5.5.0
- name: oomkill
  metrics:
    counters:
      - name: oom_kills
        help: Count global and cgroup level OOMs
        perf_map: events
        labels:
          - name: cgroup_path
            size: 8
            decoders:
              - name: uint
              - name: cgroup
          - name: global_oom
            size: 1
            decoders:
              - name: uint
  kprobes:
    oom_kill_process: count_ooms
  code: |
    #include <uapi/linux/ptrace.h>
    #include <linux/oom.h>
    #include <linux/memcontrol.h>
    // we'll use "BPF_PERF_OUTPUT" map type here to avoid unbound cardinality
    BPF_PERF_OUTPUT(events);
    struct data_t {
        u64 cgroup_id;
        u8 global_oom;
    };
    void count_ooms(struct pt_regs *ctx, struct oom_control *oc, const char *message) {
        struct data_t data = {};
        struct mem_cgroup *mcg = oc->memcg;
        if (!mcg) {
            data.global_oom = 1;
            events.perf_submit(ctx, &data, sizeof(data));
            return;
        }
        data.cgroup_id = mcg->css.cgroup->kn->id;
        events.perf_submit(ctx, &data, sizeof(data));
    }
{{- end }}
