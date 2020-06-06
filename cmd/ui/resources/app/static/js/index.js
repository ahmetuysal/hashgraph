asticode.loader.init();
asticode.modaler.init();
asticode.notifier.init();

// map of node signatures -> Kanva Group
const nodeSignatureToVisualMap = new Map();
const peerAddressToXMap = new Map();
let peerAddressesToNamesObj;
const transactionsDiv = document.getElementById("transactions");

const stage = new Konva.Stage({
  container: "konva-container", // id of container <div>
  draggable: true,
  dragBoundFunc: (pos) => {
    console.log(pos);
    return {
      x: 0,
      y: pos.y < 0 ? 0 : pos.y,
    };
  },
  width: 700,
  height: 700,
});

const eventsLayer = new Konva.Layer();

// TODO: this can be optimized (idea: keep a map to store which event is waiting who)
// if an event is received before its parents, we need to queue it
let queuedEvents = [];

let stageWidth = stage.width();
let stageHeight = stage.height();
const margin = 40;

const initCanvas = (peers) => {
  peerAddressesToNamesObj = peers;
  const peerAddressesToNames = Object.entries(peers);
  const nodeCount = peerAddressesToNames.length;
  const textSize = 12;

  const peerLinesLayer = new Konva.Layer();
  const nodeLineWidth = 5;
  const lineSpacing =
    (stageWidth - margin * 2 - nodeLineWidth * nodeCount) / (nodeCount - 1);

  for (var i = 0; i < nodeCount; i++) {
    const peerText = new Konva.Text({
      x: margin + (lineSpacing + nodeLineWidth) * i,
      y: 0,
      text: peerAddressesToNames[i][1],
      fontSize: textSize,
      fontFamily: "Calibri",
      fill: "black",
    });

    peerText.y(stageHeight - peerText.height());
    // to align text in the middle of the screen, we can set the
    // shape offset to the center of the text shape after instantiating it
    peerText.offsetX(peerText.width() / 2);

    const peerLine = new Konva.Line({
      points: [
        margin + (lineSpacing + nodeLineWidth) * i,
        -100 * stageHeight,
        margin + (lineSpacing + nodeLineWidth) * i,
        stageHeight - peerText.height(),
      ],
      stroke: "black",
      strokeWidth: nodeLineWidth,
    });

    peerLinesLayer.add(peerText);
    peerLinesLayer.add(peerLine);

    peerAddressToXMap.set(
      peerAddressesToNames[i][0],
      margin + (lineSpacing + nodeLineWidth) * i
    );
  }
  stage.add(peerLinesLayer);
  stage.add(eventsLayer);
};

const calculateColor = (event) => {
  if (event.round_received !== 0) {
    // consensus nodes are darker
    if (event.is_witness === false) {
      return "#5a5a5a";
    }
    if (event.is_fame_decided === false) {
      return "#a81616";
    }
    if (event.is_famous === true) {
      return "#158a16";
    } else {
      return "#167994";
    }
  } else {
    if (event.is_witness === false) {
      return "#9b9b9b";
    }
    if (event.is_fame_decided === false) {
      return "#f00000";
    }

    if (event.is_famous === true) {
      return "#15ad17";
    } else {
      return "#14a4cc";
    }
  }
};

// this returns true if event is displayed, false if it is queued
const displayEvent = (event) => {
  /*
    Event is a JSON object
    string        `json:"owner"`
	string        `json:"signature"`
    string        `json:"self_parent_hash"`	
    string        `json:"other_parent_hash"`
    time.Time     `json:"timestamp"`        
    []Transaction `json:"transactions"`
	uint32        `json:"round"`  
	bool          `json:"is_witness"`
	bool          `json:"is_famous"`
	bool          `json:"is_fame_decided"`
	uint32        `json:"round_received"`
	time.Time     `json:"consensus_timestamp"`
    time.Duration `json:"latency"`
    
    Transaction is a JSON object
    string  `json:"sender_address"`
	string  `json:"receiver_address"`
	float64 `json:"amount"`
    */

  if (nodeSignatureToVisualMap.get(event.signature) === undefined) {
    // we have a new event, visualize it
    if (
      event.self_parent_hash === undefined ||
      event.self_parent_hash === null ||
      event.self_parent_hash === ""
    ) {
      // it is an initial event, draw it at the bottom
      const eventX = peerAddressToXMap.get(event.owner);
      const eventY = stageWidth - 40;

      const eventVisualGroup = new Konva.Group({
        x: eventX,
        y: eventY,
      });

      const circle = new Konva.Circle({
        radius: 20,
        fill: calculateColor(event),
        stroke: "black",
        strokeWidth: 4,
      });

      const text = new Konva.Text({
        text: event.round,
        fontSize: 24,
        fontFamily: "Calibri",
        fill: "black",
      });

      text.offsetX(text.width() / 2);
      text.offsetY(text.height() / 2);

      eventVisualGroup.add(circle);
      eventVisualGroup.add(text);

      eventsLayer.add(eventVisualGroup);
      eventsLayer.draw();
      nodeSignatureToVisualMap.set(event.signature, eventVisualGroup);
      return true;
    } else if (
      nodeSignatureToVisualMap.get(event.self_parent_hash) === undefined ||
      nodeSignatureToVisualMap.get(event.other_parent_hash) === undefined
    ) {
      // parents are not drawn yet, add to queue
      queuedEvents.push(event);
      return false;
    } else {
      const eventX = peerAddressToXMap.get(event.owner);
      // find the parents location, calculate this ones location and draw
      const selfParentY = nodeSignatureToVisualMap
        .get(event.self_parent_hash)
        .y();

      const otherParentX = nodeSignatureToVisualMap
        .get(event.other_parent_hash)
        .x();
      const otherParentY = nodeSignatureToVisualMap
        .get(event.other_parent_hash)
        .y();
      const eventY = Math.min(selfParentY - 45, otherParentY - 10);
      const eventVisualGroup = new Konva.Group({
        x: eventX,
        y: eventY,
      });

      const arrowOffset = otherParentX > eventX ? -20 : 20;

      const arrow = new Konva.Arrow({
        points: [
          otherParentX + arrowOffset,
          otherParentY,
          eventX - arrowOffset,
          eventY,
        ],
        pointerLength: 10,
        pointerWidth: 10,
        fill: "black",
        stroke: "black",
        strokeWidth: 4,
      });

      const circle = new Konva.Circle({
        radius: 20,
        fill: calculateColor(event),
        stroke: "black",
        strokeWidth: 4,
      });

      const text = new Konva.Text({
        text: event.round,
        fontSize: 24,
        fontFamily: "Calibri",
        fill: "black",
      });

      text.offsetX(text.width() / 2);
      text.offsetY(text.height() / 2);

      eventVisualGroup.add(circle);
      eventVisualGroup.add(text);
      eventsLayer.add(arrow);
      eventsLayer.add(eventVisualGroup);
      eventsLayer.draw();
      nodeSignatureToVisualMap.set(event.signature, eventVisualGroup);
      return true;
    }
  } else {
    // we already visualized this event, update its state
    const visual = nodeSignatureToVisualMap.get(event.signature);
    visual.children[0].fill(calculateColor(event)); // circle

    if (event.transactions !== undefined && event.transactions !== null) {
      event.transactions.forEach((transaction) => {
        const text = document.createTextNode(
          peerAddressesToNamesObj[transaction.sender_address] +
            " -> " +
            peerAddressesToNamesObj[transaction.receiver_address] +
            ": " +
            transaction.amount.toFixed(2)
        );
        transactionsDiv.appendChild(text);
        transactionsDiv.appendChild(document.createElement("br"));
      });
    }
    return true;
  }
};

// Wait for astilectron to be ready
document.addEventListener("astilectron-ready", function () {
  astilectron.onMessage(function (message) {
    switch (message.name) {
      case "event":
        let result = displayEvent(message.payload);
        while (result) {
          const qSize = queuedEvents.length;
          queuedEvents = queuedEvents.filter((e) => !displayEvent(e));
          result = qSize > queuedEvents.length;
        }
        break;
      case "peers":
        initCanvas(message.payload);
        break;
    }
  });
});
