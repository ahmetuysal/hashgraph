asticode.loader.init();
asticode.modaler.init();
asticode.notifier.init();

const initCanvas = (peers) => {
    const canvas = document.getElementById('canvas');
    const ctx = canvas.getContext('2d');

    const peerNames = Object.values(peers)

    const nodeCount = peerNames.length;
    const textSize = 12;

    ctx.font = `${textSize}px serif`;

    const canvasWidth =  canvas.width;
    const canvasHeight = canvas.height;
    const nodeLineWidth = 2;
    const margin = 20;
    const lineSpacing = (canvasWidth - margin * 2 - nodeLineWidth * nodeCount) / (nodeCount-1);

    for (var i = 0; i < nodeCount; i++) {
        ctx.fillRect(margin + (lineSpacing + nodeLineWidth) * i, 0, nodeLineWidth, canvasHeight - textSize);
        const textMeasurement = ctx.measureText(peerNames[i]);
        ctx.fillText(peerNames[i], margin - textMeasurement.width/2 + (lineSpacing + nodeLineWidth) * i, canvasHeight);
    }
}

// Wait for astilectron to be ready
document.addEventListener('astilectron-ready', function() {
    astilectron.onMessage(function(message) {
        switch (message.name) {
            case "event":
                console.log(message.payload);
                break;
            case "peers":
                initCanvas(message.payload);
                break;
        }
    });
})



const transactionsDiv = document.getElementById('transactions');

addRandomText = () => {
    var text = document.createTextNode("Ahmet -> Erhan 100₺");
    transactionsDiv.appendChild(text);
    transactionsDiv.appendChild(document.createElement("br"));
    requestAnimationFrame(addRandomText, 1000);
}

requestAnimationFrame(addRandomText, 1000);
