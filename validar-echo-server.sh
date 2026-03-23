MSG="Message N°1"
RESULT=$(printf "%s\n" "$MSG" | docker run --rm -i --network tp0_testing_net busybox nc -i 1 -w 2 server 12345)

if [ "$RESULT" = "$MSG" ]; then
    echo "action: test_echo_server | result: success"
else
    echo "action: test_echo_server | result: fail"
    exit 1
fi