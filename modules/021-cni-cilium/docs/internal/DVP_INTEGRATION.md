# Import/export http endpoints example usage

Conntrack format version will be passed back to client of export http endoint by the header: "Cilium-Conntrack-Export-Version: XYZ"

curl --unix-socket /var/run/cilium/cilium.sock "http://localhost/v1/conntrack/export?ip4=10.0.1.3" --output exported_conntracks.bin

curl --unix-socket /var/run/cilium/cilium.sock -X POST -H "Content-Type: application/octet-stream" -H "Cilium-Conntrack-Export-Version: XYZ" --data-binary @exported_conntracks.bin "http://localhost/v1/conntrack/import"
