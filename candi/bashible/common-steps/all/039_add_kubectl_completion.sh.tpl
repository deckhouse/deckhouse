sed -i 's/\# \"\\e\[5~\": history-search-backward/\"\\e\[5~\": history-search-backward/' /etc/inputrc
sed -i 's/^\# \"\\e\[6~\": history-search-forward/\"\\e\[6~\": history-search-forward/' /etc/inputrc

sed -i 's/\#force_color_prompt=yes/force_color_prompt=yes/' /root/.bashrc
sed -i 's/01;32m/01;31m/' /root/.bashrc

kubectl completion bash >/etc/bash_completion.d/kubectl
