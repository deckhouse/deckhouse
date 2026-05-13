cd ~/witness/test
./compact.sh
./defrag.sh
./s-1-run-range.sh all
./s-2-run-txn-mixed.sh low
./compact.sh
./s-2-run-txn-mixed.sh high
./compact.sh
./defrag.sh
./s-3-run-put.sh low
./compact.sh
./s-3-run-put.sh high
./compact.sh
./defrag.sh
