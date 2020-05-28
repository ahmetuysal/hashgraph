## To test in localhost, we create separate folders for each peer.

echo "Removing existing files and creating new ones..."
for i in {1..5} # The last number is the number of peers
do
    rm -r "$i";
    mkdir "$i";
    cp "./peer.go" "./$i/";
    cp "./peers.txt" "./$i/";
done
echo "Succesfully created folders for peers and copied files in them."


