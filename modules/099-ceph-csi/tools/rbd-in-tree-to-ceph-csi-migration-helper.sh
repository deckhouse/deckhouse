#!/usr/bin/env bash

sample_pvc_name="$1"
target_pvc_name="$2"

sample_pvc="$(kubectl get pvc "$sample_pvc_name" -o json)"
target_pvc="$(kubectl get pvc "$target_pvc_name" -o json)"

sample_pv_name="$(jq -r '.spec.volumeName' <<< "$sample_pvc")"
target_pv_name="$(jq -r '.spec.volumeName' <<< "$target_pvc")"

sample_pv="$(kubectl get pv "$sample_pv_name" -o json)"
target_pv="$(kubectl get pv "$target_pv_name" -o json)"

pool_name="$(jq -r '.spec.csi.volumeAttributes.pool' <<< "$sample_pv")"
original_rbd_image_name="$(jq -r '.spec.rbd.image' <<< "$target_pv")"
new_rbd_image_name="$(jq -rn --arg s "$original_rbd_image_name" '$s | sub("kubernetes-dynamic-pvc-"; "csi-vol-")')"
new_rbd_image_uid="$(jq -rn --arg s "$original_rbd_image_name" '$s | sub("kubernetes-dynamic-pvc-"; "")')"
sample_rbd_image_uid="$(jq -r '.spec.csi.volumeAttributes.imageName | sub("csi-vol-"; "")' <<< "$sample_pv")"

csi_section_for_target_pv="$(jq -r --arg i "$new_rbd_image_name" '.spec.csi.volumeAttributes.imageName = $i | .spec.csi' <<< "$sample_pv")"
new_storage_class_for_target="$(jq -r '.spec.storageClassName' <<< "$sample_pvc")"
new_annotations_for_target_pvc="$(jq -r '.metadata.annotations' <<< "$sample_pvc")"
new_annotations_for_target_pv="$(jq -r '.metadata.annotations' <<< "$sample_pv")"

new_target_pvc="$(jq --argjson a "$new_annotations_for_target_pvc" --arg sc "$new_storage_class_for_target" '
  .metadata.annotations = $a |
  .spec.storageClassName = $sc |
  del(.metadata.resourceVersion) |
  del(.metadata.uid) |
  del(.metadata.creationTimestamp) |
  del(.status)
  ' <<< "$target_pvc")"

while true; do
  msg="rbd mv $pool_name/$original_rbd_image_name $pool_name/$new_rbd_image_name
Rename the rbd image in the ceph cluster and type \"yes\": "
  read -p "$msg" confirm
  if [ "$confirm" == "yes" ]; then
    break
  fi
done


echo "kubectl delete pvc $target_pvc_name
kubectl delete pv $target_pv_name"

while true; do
  read -p "PVC $target_pvc_name and PV $target_pv_name will be removed (Type yes to confirm): " confirm
  if [ "$confirm" == "yes" ]; then
    kubectl delete pvc $target_pvc_name
    kubectl delete pv $target_pv_name
    break
  fi
done

echo "kubectl create -f - <<\"END\"
$new_target_pvc
END"

while true; do
  read -p "Apply this manifest in the cluster? (Type yes to confirm): " confirm
  if [ "$confirm" == "yes" ]; then
    kubectl create -f - <<END
$new_target_pvc
END
    sleep 7
    break
  fi
done

new_target_pvc="$(kubectl get pvc "$target_pvc_name" -o json)"
new_target_pvc_metadata="$(jq -r '.metadata' <<< "$new_target_pvc")"

new_target_pv="$(jq --argjson m "$new_target_pvc_metadata" --argjson a "$new_annotations_for_target_pv" --argjson csi "$csi_section_for_target_pv" --arg sc "$new_storage_class_for_target" --arg sampleRbdImageUid "$sample_rbd_image_uid" --arg newRbdImageUid "$new_rbd_image_uid" '
  .metadata.annotations = $a |
  .spec.claimRef.resourceVersion = $m.resourceVersion |
  .spec.claimRef.uid = $m.uid |
  .spec.csi = $csi |
  .spec.storageClassName = $sc |
  .spec.persistentVolumeReclaimPolicy = "Retain" |
  .spec.csi.volumeHandle = (.spec.csi.volumeHandle | sub($sampleRbdImageUid; $newRbdImageUid)) |
  del(.spec.rbd) |
  del(.metadata.resourceVersion) |
  del(.metadata.uid) |
  del(.metadata.creationTimestamp) |
  del(.status)
  ' <<< "$target_pv")"

echo "kubectl create -f - <<\"END\"
$new_target_pv
END"

while true; do
  read -p "Apply this manifest in the cluster? (Type yes to confirm): " confirm
  if [ "$confirm" == "yes" ]; then
    kubectl create -f - <<END
$new_target_pv
END
    break
  fi
done
