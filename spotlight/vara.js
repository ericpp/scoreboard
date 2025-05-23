const Vara = function (elem, fontSource, text, properties) {
  var _this = this;
  this.elementName = elem;
  this.textsInit = [];
  if (typeof text == "string")
    this.textsInit.push({
      text: text,
    });
  else if (typeof text == "object") this.textsInit = text;
  this.texts = this.textsInit;
  this.properties = properties || {};
  this.properties.textAlign = this.properties.textAlign || "left";
  this.letterSpacing = 0;
  this.element = document.querySelector(this.elementName);

  if (!this.element) {
    console.error(`Element "${this.elementName}" not found`);
    return;
  }

  this.fontSource = fontSource;
  this.characters = {};
  this.drawnCharacters = {};
  this.totalPathLength = 0;
  this.fontSize = 24;
  this.frameRate = 1000 / 30;
  this.prevDuration = 0;
  this.completed = false;

  // Initialize promise to track completion
  this.initializing = false;
  this.initialized = false;

  this.ready = function (f) {
    _this.readyF = f;
    return _this;
  };

  this.animationEnd = function (f) {
    _this.animationEndF = f;
    return _this;
  };

  this.svg = this.createNode("svg", {
    width: "100%",
  });
  this.element.appendChild(this.svg);
  this.font = document.createElement("object");

  // Start initialization
  this.init();

  // Return this for method chaining
  return this;
};

/**
 * Initialize the Vara instance by loading the font and creating the text
 * @returns {Promise} Promise that resolves when initialization is complete
 */
Vara.prototype.init = function() {
  if (this.initializing) return;
  this.initializing = true;

  // Start initialization process
  this.getSVGData()
    .then(() => {
      this.initialized = true;
      this.initializing = false;
    })
    .catch(error => {
      this.initializing = false;
      console.error("Vara initialization failed:", error);
    });
};

/**
 * Creates SVG nodes
 * @param {string} n Name of the node to be created
 * @param {object} v Object with properties of the element
 * returns {node}
 */
Vara.prototype.createNode = function (n, v) {
  n = document.createElementNS("http://www.w3.org/2000/svg", n);
  for (var p in v)
    n.setAttributeNS(
      null,
      p.replace(/[A-Z]/g, function (m, p, o, s) {
        return "-" + m.toLowerCase();
      }),
      v[p]
    );
  return n;
};

const VaraFont = {
  loading: {},
  cache: {},

  loadFont: async function(url) {
    // Return cached font if available
    if (this.cache[url]) return this.cache[url];

    // Return in-progress loading promise if exists
    if (this.loading[url]) return await this.loading[url];

    // Start new loading process
    this.loading[url] = fetch(url)
      .then(response => {
        if (!response.ok) {
          throw new Error(`Failed to load font from ${url}: ${response.status} ${response.statusText}`);
        }
        return response.json();
      })
      .then(data => {
        this.cache[url] = data;
        delete this.loading[url];
        return data;
      })
      .catch(error => {
        delete this.loading[url]; // Clean up the loading promise on error
        console.error(`VaraFont error: ${error.message}`);
        throw error; // Re-throw to allow caller to handle
      });

    return await this.loading[url];
  }
};

/**
 * Used to extract data from the JSON data
 */
Vara.prototype.getSVGData = async function () {
  try {
    // Load the font
    const font = await VaraFont.loadFont(this.fontSource);
    this.contents = font;
    this.characters = font.c;

    // Process and create text
    this.preCreate();
    this.createText();

    // Return the contents for promise chaining
    return this.contents;
  } catch (error) {
    console.error(`Failed to load or process font: ${error.message}`);

    // Call the ready function with error if provided
    if (this.readyF) {
      this.readyF(error);
    }

    throw error;
  }
};

/**
 * Used to add or change required fields provided as arguments in the class name.
 */
Vara.prototype.preCreate = function () {
  (function () {
    if (typeof NodeList.prototype.forEach === "function") return false;
    NodeList.prototype.forEach = Array.prototype.forEach;
  })();

  // If the question mark symbol exists in the loaded JSON font, then it will be used otherwise the default question mark will be used. Any symbol that is not in the JSON is replaced by a question mark.

  this.questionMark =
    this.characters["63"] == undefined
      ? {
          paths: [
            {
              w: 8.643798828125,
              h: 14.231731414794922,
              my: 22.666500004827977,
              mx: 0,
              pw: 28.2464542388916,
              d: "m 0,0 c -2,-6.01,5,-8.64,8,-3.98,2,4.09,-7,8.57,-7,11.85",
            },
            {
              w: 1.103759765625,
              h: 1.549820899963379,
              my: 8.881500004827977,
              mx: 1,
              pw: 4.466640472412109,
              d: "m 0,0 a 0.7592,0.7357,0,0,1,0,0.735,0.7592,0.7357,0,0,1,-1,-0.735,0.7592,0.7357,0,0,1,1,-0.738,0.7592,0.7357,0,0,1,0,0.738 z",
            },
          ],
          w: 8.643798828125,
        }
      : this.characters["63"];

  // A path represnting 'space'
  this.space = {
    paths: [
      {
        d: "M0,0 l" + this.contents.p.space + " 0",
        mx: 0,
        my: 0,
        w: this.contents.p.space,
        h: 0,
      },
    ],
    w: this.contents.p.space,
  };
  // analyseWidth() will return an object containing breakPoints (positions where new line occurs)
  var widthAnalysis = this.analyseWidth();
  for (var k = 0; k < this.texts.length; k++) {
    var alteredText = [];
    var textObj =
      typeof this.texts[k] == "string"
        ? {
            text: this.texts[k],
          }
        : this.texts[k];
    var textArray = textObj.text;
    for (var x = 0; x < textArray.length; x++) {
      var count = 0;
      var prevPos = 0;
      var text = textArray[x];
      /*
            If there are new lines, the string is breaked at these positions and are appended to an array.
            The value of text property in 'texts' object is replaced by this new array.
            This is done because each array inside the text property is considered as a new line when creating the text.
            */
      if (widthAnalysis.breakPoints[k][x].length > 0) {
        var newText;
        var len = widthAnalysis.breakPoints[k][x].length;
        for (var j = 0; j <= len; j++) {
          var pos = widthAnalysis.breakPoints[k][x][j];
          if (widthAnalysis.breakPoints[k][x][len - 1] != text.length)
            widthAnalysis.breakPoints[k][x].push(text.length);
          var prev =
            widthAnalysis.breakPoints[k][x][j - 1] == undefined
              ? 0
              : widthAnalysis.breakPoints[k][x][j - 1];
          newText = text.slice(prev, pos).replace(/^\s+/g, "");
          alteredText.push(newText);
        }
      } else {
        alteredText.push(textArray[x]);
      }
    }
    this.texts[k].text = alteredText;
  }
};

/**
 * creates and add the elements (letters) into the svg element.
 */
Vara.prototype.createText = function () {
  var _this = this;
  var containerGroup = this.svg;
  var correction = 0;
  var lho = 0;
  var fontSize;
  // if lineHeight is provided for a specfic paragraph, then it will be used. If it is not specified then the global 'properties' object is searched for lineHeight, if it is provided then it will be used. Otherwise the default lineHeight provided in the JSON font object will be used.
  var lineHeight =
    this.properties.lineHeight == undefined
      ? this.contents.p.lh
      : this.properties.lineHeight;
  this.properties.autoAnimation =
    this.properties.autoAnimation == undefined
      ? true
      : this.properties.autoAnimation;
  var prevOuterHeight = 0;
  var prevDuration = 0;
  // incrementOuterHeight determines whether to add new paragraphs below the previous paragraph.
  var incrementOuterHeight = true;
  for (var j = 0; j < this.texts.length; j++) {
    var drawnPart = [];
    var lhi = 0;
    fontSize = this.texts[j].fontSize;
    var bsw = this.texts[j].strokeWidth;
    var color =
      this.texts[j].color == undefined
        ? this.properties.color == undefined
          ? "black"
          : this.properties.color
        : this.texts[j].color;
    var duration =
      this.texts[j].duration == undefined
        ? this.properties.duration == undefined
          ? 2000
          : this.properties.duration
        : this.texts[j].duration;
    this.texts[j].duration = duration;
    var id = this.texts[j].id == undefined ? j : this.texts[j].id;
    this.texts[j].delay =
      this.texts[j].delay == undefined ? 0 : this.texts[j].delay;
    this.prevDuration += this.texts[j].delay;
    /* If the y coordinate of the paragraph is given and the property `fromCurrentPosition` is set to false then the text is drawn at the 	 absolute position given.
           If the y coordinate of the paragraph is given and the property `fromCurrentPosition` is not set or set to true then the text is drawn relative to the previous element. That is if the y values is given as 50, then the element will be created 50px below the previous one.
        */
    if (this.texts[j].fromCurrentPosition == undefined)
      this.texts[j].fromCurrentPosition = {
        x: true,
        y: true,
      };
    if (this.texts[j].y == undefined) incrementOuterHeight = true;
    else if (
      this.texts[j].fromCurrentPosition.y == undefined ||
      this.texts[j].fromCurrentPosition.y
    )
      incrementOuterHeight = true;
    else incrementOuterHeight = false;
    var tc = color;
    var textAlign =
      this.texts[j].textAlign == undefined
        ? this.properties.textAlign
        : this.texts[j].textAlign;
    lineHeight =
      this.texts[j].lineHeight == undefined
        ? lineHeight
        : this.texts[j].lineHeight / fontSize;
    // outerLayer is used to contain the entire paragraph.
    var outerLayer = this.createNode("g", {
      class: "outer",
      transform: "translate(0,0)",
      "data-text": this.texts[j].text,
    });
    containerGroup.appendChild(outerLayer);
    lho = prevOuterHeight;
    var ph = 0;
    for (var x = 0; x < this.texts[j].text.length; x++) {
      var cWidth = 0;
      // used to contain the current line.
      var fg = this.createNode("g");
      outerLayer.appendChild(fg);
      var largestHeight = 0;
      var lFH = 0;
      for (var i = 0; i < this.texts[j].text[x].length; i++) {
        var text = this.texts[j].text[x][i];
        // Group which contains the letter
        var cGroup = this.createNode("g");
        fg.appendChild(cGroup);
        var correction = 0;
        var paths =
          this.characters[text.charCodeAt(0)] != undefined || text == " "
            ? text == " "
              ? this.space.paths
              : this.characters[text.charCodeAt(0)].paths
            : this.questionMark.paths;
        if (text == " ") color = "transparent";
        else color = tc;
        paths.forEach(function (e, ei) {
          var path = _this.createNode("path", {
            d: e.d,
            "stroke-width": bsw,
            stroke: color,
            fill: "none",
            "stroke-linecap": _this.contents.p.slc,
            "stroke-linejoin": _this.contents.p.slj,
          });
          cGroup.appendChild(path);
          path.setAttribute(
            "transform",
            "translate(" + e.mx + "," + -e.my + ")"
          );
          largestHeight =
            largestHeight > e.my - path.getBBox().y
              ? largestHeight
              : e.my - path.getBBox().y;
          if (ei == 0) lFH = largestHeight;
          path.style.opacity = 0;
          /*
                    Adding 2 and 1 to solve a bug on Chrome or (Firefox and Edge). (I don't know which browsers are showing the correct output)
                    https://stackoverflow.com/questions/53626733/dot-appears-before-line-with-round-linecap-ff-ie/53627545#53627545
                    */
          path.style.strokeDasharray =
            path.getTotalLength() + " " + (path.getTotalLength() + 2);
          path.style.strokeDashoffset = path.getTotalLength() + 1;
        });
        drawnPart.push(cGroup);
        var correction = cGroup.getBBox().x * fontSize;
        var letterSpacing = this.texts[j].letterSpacing;
        if (typeof letterSpacing === "object") {
          letterSpacing =
            letterSpacing[text] === undefined
              ? letterSpacing["global"] === undefined
                ? 0
                : letterSpacing["global"]
              : letterSpacing[text];
        }
        if (cGroup.getBBox().width < this.texts[j].minWidth)
          correction =
            correction - (this.texts[j].minWidth - cGroup.getBBox().width) / 2;

        cGroup.setAttribute(
          "transform",
          "translate(" +
            (cWidth - correction + letterSpacing) +
            ",0)  scale(" +
            fontSize +
            ")"
        );

        cWidth += cGroup.getBBox().width * fontSize + letterSpacing;

        if (cGroup.getBBox().width < this.texts[j].minWidth)
          cWidth += this.texts[j].minWidth - cGroup.getBBox().width;
      }
      var fgBox = fg.getBBox();
      fg.setAttribute(
        "transform",
        "translate(" + bsw * fontSize + "," + (-fgBox.y + bsw * fontSize) + ")"
      );
      var alignX = 0;
      if (textAlign == "center") {
        alignX = (this.svg.getBoundingClientRect().width - fgBox.width) / 2;
      } else if (textAlign == "right") {
        alignX = this.svg.getBoundingClientRect().width - fgBox.width;
      }
      this.setPosition(fg, {
        x: alignX,
        y: lhi + this.contents.p.tf * fontSize - largestHeight,
      });
      // Increment line position as each line is created.
      lhi += lineHeight * fontSize;
      ph +=
        this.contents.p.tf * fontSize + (this.contents.p.tf * fontSize - lFH);
    }
    if (
      this.texts[j].y == undefined ||
      this.texts[j].fromCurrentPosition.y == true
    ) {
      this.setPosition(outerLayer, {
        y: lho,
      });
    }
    if (
      this.texts[j].fromCurrentPosition != undefined &&
      this.texts[j].fromCurrentPosition.y
    ) {
      prevOuterHeight += this.texts[j].y == undefined ? 0 : this.texts[j].y;
    }
    this.setPosition(
      outerLayer,
      {
        x: this.texts[j].x,
        y: this.texts[j].y,
      },
      this.texts[j].fromCurrentPosition
    );
    if (incrementOuterHeight) prevOuterHeight += ph;
    if (this.drawnCharacters[id] != undefined) id = j;
    this.drawnCharacters[id] = {
      characters: drawnPart,
      queued: this.texts[j].queued,
      container: outerLayer,
      index: j,
    };
    if (
      (this.texts[j].autoAnimation == undefined ||
        this.texts[j].autoAnimation) &&
      this.properties.autoAnimation
    ) {
      _this.draw(id, duration);
      if (this.texts[j].queued == undefined || this.texts[j].queued) {
        _this.prevDuration += duration;
      }
    }
  }
  this.completed = true;
  this.svg.setAttribute(
    "height",
    this.svg.getBBox().height + this.svg.getBBox().y + 10
  );
  if (this.readyF) this.readyF();
};

Vara.prototype.playAll = function () {
  // If not initialized, wait for initialization or initialize
  if (!this.initialized) {
    if (!this.initializing) {
      this.init();
    }

    console.warn("Vara is still initializing. Animation will start when ready.");

    // Setup a one-time ready function if not already set
    const origReadyFn = this.readyF;
    this.readyF = (error) => {
      // First call original ready function if exists
      if (origReadyFn) origReadyFn(error);

      // Play all animations if initialization was successful
      if (!error) this.playAll();

      // Restore original ready function
      this.readyF = origReadyFn;
    };

    return;
  }

  // Reset duration counter
  this.prevDuration = 0;

  // Play each text
  for (var j = 0; j < this.texts.length; j++) {
    var duration = this.texts[j].duration;
    var id = this.texts[j].id == undefined ? j : this.texts[j].id;
    this.prevDuration += this.texts[j].delay;

    // Ensure the element exists before trying to animate it
    if (this.drawnCharacters[id]) {
      this.draw(id, duration);
      if (this.texts[j].queued == undefined || this.texts[j].queued) {
        this.prevDuration += duration;
      }
    } else {
      console.warn(`ID:${id} not found. Animation skipped.`);
    }
  }

  return this; // For method chaining
};

/**
 * Animates the drawing of the text
 * @param {int} id index or id of the paragraph to be animated.
 * @param {int} dur Duration of the animation in milliseconds
 * @returns {Vara} The Vara instance for chaining
 */
Vara.prototype.draw = function (id, dur) {
  /*
    This will iterate through each character, finds its path length and the total duration is divided with respect to its path length.
    */
  var _this = this;

  // Check if the element exists
  if (this.drawnCharacters[id] == undefined) {
    console.warn("ID:`" + id + "` not found. Animation skipped");
    console.trace();
    return this;
  }

  // Ensure we have valid duration
  var duration = dur === undefined
    ? this.texts[this.drawnCharacters[id].index].duration
    : dur;

  duration = Math.max(0, Number(duration) || 0);

  var pathLength = this.getSectionPathLength(id);
  var delay = 0;
  var queued = this.drawnCharacters[id].queued == undefined
    ? true
    : this.drawnCharacters[id].queued;
  var timeOut = queued ? this.prevDuration : 1;

  // Track timeout ID for clean handling
  var timeoutId = setTimeout(function () {
    // Check if characters array exists (might have been reset)
    if (!_this.drawnCharacters[id] || !_this.drawnCharacters[id].characters) {
      console.warn("Characters for ID:`" + id + "` no longer exist. Animation skipped");
      return;
    }

    _this.drawnCharacters[id].characters.forEach(function (i) {
      if (!i) return; // Skip if element is null/undefined

      var paths = i.querySelectorAll("path");
      if (!paths || paths.length === 0) return;

      paths.forEach(function (j) {
        var currentDuration = (parseFloat(j.style.strokeDashoffset) / pathLength) * duration;
        j.style.opacity = 1;
        _this.animate(j, currentDuration, delay, 0);
        delay += currentDuration;
      });
    });

    // Call animation end after all elements are animated
    setTimeout(function () {
      if (_this.animationEndF) {
        _this.animationEndF(id, _this.drawnCharacters[id]);
      }
    }, delay);
  }, timeOut);

  // Store timeout ID for potential cancellation
  this.drawnCharacters[id].timeoutId = timeoutId;

  return this;
};

/**
 * Gets the drawn character information by ID
 * @param {string|number} id ID of the paragraph
 * @returns {object|null} Information about the drawn text or null if not found
 */
Vara.prototype.get = function (id) {
  if (this.drawnCharacters[id] == undefined) {
    console.warn("ID:`" + id + "` not found.");
    console.trace();
    return null;
  }

  return this.drawnCharacters[id];
};

/**
 * Handles animation of the stroke dashoffset
 * @param {Node} elem Element to be animated
 * @param {int} duration Duration of the animation
 * @param {int} delay Delay
 * @param {int} final Final position of the stroke Dashoffset
 */
Vara.prototype.animate = function (elem, duration, delay, final) {
  if (!elem || !elem.style) {
    console.warn("Animation element is invalid");
    return;
  }

  var _this = this;
  final = Number(final) ||.0;
  duration = Math.max(0, Number(duration) || 0);
  delay = Math.max(0, Number(delay) || 0);

  // Store the current animation timer on the element for potential cancellation
  if (elem._varaAnimationTimer) {
    clearTimeout(elem._varaAnimationTimer);
    delete elem._varaAnimationTimer;
  }

  elem._varaAnimationTimer = setTimeout(function () {
    if (!elem || !elem.style) {
      // Element may have been removed
      return;
    }

    var start = new Date().getTime();
    var initial = parseFloat(elem.style.strokeDashoffset);

    // Clean up any previous interval
    if (elem._varaAnimationInterval) {
      clearInterval(elem._varaAnimationInterval);
    }

    elem._varaAnimationInterval = setInterval(function () {
      if (!elem || !elem.style) {
        // Element may have been removed during animation
        clearInterval(elem._varaAnimationInterval);
        return;
      }

      var step = Math.min(1, (new Date().getTime() - start) / duration);
      var current = initial + step * (final - initial);

      elem.style.strokeDashoffset = current;

      if (step >= 1) {
        clearInterval(elem._varaAnimationInterval);
        delete elem._varaAnimationInterval;
      }
    }, _this.frameRate);
  }, delay);
};

/**
 * Gets the path length of the entire paragraph
 * @param {int} id Index (id) of the paragraph
 * @returns {number} The total path length
 */
Vara.prototype.getSectionPathLength = function (id) {
  if (!this.drawnCharacters[id] || !this.drawnCharacters[id].characters) {
    console.warn(`Cannot calculate path length for ID:${id} - not found or has no characters`);
    return 0;
  }

  var _this = this;
  this.totalPathLength = 0;

  try {
    this.drawnCharacters[id].characters.forEach(function (i) {
      if (!i) return; // Skip if null/undefined

      const paths = i.querySelectorAll("path");
      if (!paths) return;

      paths.forEach(function (j) {
        if (!j) return; // Skip if path is null

        try {
          _this.totalPathLength += j.getTotalLength();
        } catch (error) {
          console.warn(`Error calculating path length: ${error.message}`);
        }
      });
    });
  } catch (error) {
    console.error(`Error in getSectionPathLength: ${error.message}`);
  }

  return this.totalPathLength || 0.1; // Return at least a small value to avoid division by zero
};

/**
 * analyseWidth scans through each element and determines the position of each characters.
 * @returns {object} Object containing width and an array of breakPoints.
 */
Vara.prototype.analyseWidth = function () {
  var width = 0;
  var canvasOriginalWidth = this.svg.getBoundingClientRect().width;
  // breakPoints will store the position where a new line appears.
  var breakPoints = [];
  /* baseLetter is a reference character.
       The character representing the given letter will be appended to the container element as a span element.
       Then its width is calculated and is considered as the refernce width.
       All other characters are scaled according to the reference width.
    */
  var baseLetter =
    this.characters["97"] == undefined
      ? Object.keys(this.characters)[
          Math.round(Math.random() * Object.keys(this.characters).length - 1)
        ]
      : "97";
  var baseFont = document.createElement("span");
  this.element.appendChild(baseFont);
  baseFont.style.opacity = 0;
  baseFont.style.position = "absolute";
  baseFont.innerHTML = String.fromCharCode(baseLetter);
  /*
    This element is added to calculate the width a dot takes with space on either sides.
    This width is then set as the min width because otherwise small characters like dot, comma etc will stick to the previous characters making it hard to see.
    Required width is added to either sides of the characters having width less than the min width
    */
  var dot = document.createElement("span");
  this.element.appendChild(dot);
  dot.style.opacity = 0;
  dot.style.position = "absolute";
  dot.innerHTML = " . ";
  for (var j = 0; j < this.texts.length; j++) {
    var textArray,
      textObj = this.texts[j];
    // If the text is given as a string, it is converted into an array
    if (typeof textObj.text == "string") textArray = [textObj.text];
    else textArray = textObj.text;
    this.texts[j].text = textArray;
    this.texts[j].letterSpacing =
      this.texts[j].letterSpacing == undefined
        ? this.properties.letterSpacing == undefined
          ? 0
          : this.properties.letterSpacing
        : this.texts[j].letterSpacing;
    this.texts[j].strokeWidth =
      this.texts[j].strokeWidth == undefined
        ? this.properties.strokeWidth == undefined
          ? this.contents.p.bsw
          : this.properties.strokeWidth
        : this.texts[j].strokeWidth;
    // Whether to break the word on overflow.
    var breakWord =
      this.texts[j].breakWord == undefined
        ? this.properties.breakWord == undefined
          ? false
          : this.properties.breakWord
        : this.texts[j].breakWord;
    var originalBreakWord = breakWord;
    var fontSize =
      textObj.fontSize == undefined
        ? this.properties.fontSize == undefined
          ? this.fontSize
          : this.properties.fontSize
        : textObj.fontSize;
    baseFont.style.fontSize = fontSize + "px";
    dot.style.fontSize = fontSize + "px";
    var scale =
      baseFont.getBoundingClientRect().width / this.characters[baseLetter].w;
    this.texts[j].minWidth = dot.getBoundingClientRect().width;

    // if a width is specified, overflow, textAlign and other properties will be calculated with respect to the given width. Otherwise the width of the SVG element will be used.
    var canvasWidth =
      this.texts[j].width == undefined
        ? canvasOriginalWidth
        : this.texts[j].width;
    var bp1 = [];
    var increment;
    var inx = this.texts[j].x == undefined ? 0 : this.texts[j].x;
    this.trueFontSize = fontSize;
    this.texts[j].fontSize = scale;

    var letterSpacing = this.texts[j].letterSpacing;
    /*
        Each character is iterated and is added to the variable lWidth. If the x coordinate of the paragraph is given lWidth will be initialized with the x coordinate.
        if the width exceeds the canvasWidth, then its array index value is appended to the breakPoints array.
        breakPoints represents the indices where a new line appears.
        */
    for (var x = 0; x < textArray.length; x++) {
      var lWidth = inx;
      var bp2 = [];
      var text = textArray[x];
      var lastSpace = 0;
      for (var i = 0; i < text.length; i++) {
        if (typeof letterSpacing === "object") {
          letterSpacing =
            letterSpacing[text[i]] === undefined
              ? letterSpacing["global"] === undefined
                ? 0
                : letterSpacing["global"]
              : letterSpacing[text[i]];
        }
        if (this.characters[text[i].charCodeAt(0)] != undefined) {
          increment = this.characters[text[i].charCodeAt(0)].w * scale;
          if (increment < this.texts[j].minWidth)
            increment +=
              (scale *
                (this.texts[j].minWidth -
                  this.characters[text[i].charCodeAt(0)].w)) /
              2;
          increment += letterSpacing;
        } else {
          if (text[i] == " ") {
            increment = this.space.w * scale;
            lastSpace = lWidth;
          } else increment = this.questionMark.w * scale + letterSpacing;
        }
        increment += this.texts[j].strokeWidth * scale;
        if (lWidth + increment >= canvasWidth) {
          if (lastSpace == 0) breakWord = true;
          var pos = i;
          if (text[i] != " " && !breakWord) {
            pos = text.slice(0, pos + 1).search(/\S+$/);
          }
          bp2.push(pos);
          lWidth = inx + lWidth - lastSpace;
        } else {
          width += increment;
          lWidth += increment;
        }
      }
      bp1.push(bp2);
    }
    breakPoints.push(bp1);
  }
  baseFont.parentNode.removeChild(baseFont);
  dot.parentNode.removeChild(dot);
  return {
    width: width,
    breakPoints: breakPoints,
  };
};

/**
 * Sets the position of the node
 * @param {Node} e Element
 * @param {object} obj Object with the x and y coordinates
 * @param {object} relative Object with properties x and y determining whether the respective coordinate is relative to the previous element position
 */
Vara.prototype.setPosition = function (e, obj, relative) {
  var relative =
    relative == undefined
      ? {
          x: false,
          y: false,
        }
      : relative;
  relative.x = relative.x == undefined ? false : relative.x;
  relative.y = relative.y == undefined ? false : relative.y;
  var p = e.transform.baseVal.consolidate().matrix;

  var x = p.e,
    y = p.f;
  if (obj.x != undefined) {
    if (relative.x) x = x + obj.x;
    else x = obj.x;
  }
  if (obj.y != undefined) {
    if (relative.y) y = y + obj.y;
    else y = Math.max(obj.y - e.getBBox().y, 0);
  }

  var translate = this.svg.createSVGTransform();
  translate.setTranslate(x, y);

  e.transform.baseVal.replaceItem(translate, 0);

  /*
	This was the initial way i chose to set position, but it is not supported in safari.
	e.transform.baseVal.consolidate().setTranslate(x, y);

	*/
};

/**
 * Resets the instance and allows redrawing with new text
 * @param {string|object} text New text to draw
 * @param {object} properties Optional properties to override
 * @returns {Vara} The Vara instance for chaining
 */
Vara.prototype.reset = function (text, properties) {
  // Save the container element and font source
  var elementName = this.elementName;
  var fontSource = this.fontSource;

  // Clear SVG content
  if (this.svg) {
    while (this.svg.firstChild) {
      this.svg.removeChild(this.svg.firstChild);
    }
  }

  // Reset instance properties
  this.textsInit = [];
  if (typeof text == "string")
    this.textsInit.push({
      text: text,
    });
  else if (typeof text == "object") this.textsInit = text;
  this.texts = this.textsInit;

  // Update properties if provided
  if (properties) {
    this.properties = Object.assign({}, this.properties, properties);
  }

  this.properties.textAlign = this.properties.textAlign || "left";

  // Reset drawing state
  this.drawnCharacters = {};
  this.totalPathLength = 0;
  this.prevDuration = 0;
  this.completed = false;
  this.initialized = false;
  this.initializing = false;

  // Process and draw the new text
  this.init();

  return this;
};

if (typeof module !== "undefined") {
  module.exports = Vara;
} else {
  window.Vara = Vara;
}