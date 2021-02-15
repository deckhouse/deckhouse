if [[ "$FIRST_BASHIBLE_RUN" == "yes" ]]; then
  hostnamectl set-hostname $(hostname -s)
fi
