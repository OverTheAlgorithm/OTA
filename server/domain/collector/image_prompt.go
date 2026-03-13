package collector

type ImagePrompt string

const (
	commonHeader ImagePrompt = `Create a single, impactful illustration for a news thumbnail.

STRICT RULES (must follow):
• ABSOLUTELY NO TEXT, LETTERS, NUMBERS, OR WORDS anywhere in the image. Not on signs, labels, speech bubbles, clothing, or any surface. The image must be purely visual with zero written characters.
• Keep the image CLEAN. Serious or heavy topics are fine, but the visual must never be grotesque or disgusting. Avoid: dripping fluids from bodies, exaggerated open mouths showing food/teeth, needles/syringes piercing skin, body horror, gore, or visceral imagery that provokes disgust. The tone can be serious, somber, or dramatic — but always visually clean.

Based on the following news topic:
%s

Select the most visually striking moment, person, or object that represents the core of the story`
	colorPencilStyle ImagePrompt = `.

Illustration style:
• Colored pencil drawing style
• Hand-sketched, editorial illustration
• Soft pencil shading and visible colored pencil texture
• Clean white background
• Minimal composition
• Focus on one central subject
• Simple and symbolic visual storytelling
• Light, natural colors similar to newspaper editorial sketches

Composition:
• One main subject only (person, object, or scene)
• Centered composition
• No complex background
• No text

`
	paperArtStyle ImagePrompt = `
Illustration style:
• Origami papercraft style
• Everything made from folded paper
• Sharp folds and geometric paper shapes
• Delicate papercraft textures
• Layered paper structures
• Stop-motion animation aesthetics
• Soft studio lighting
• Craft design feeling
• Minimal but detailed paper construction

Composition:
• One main subject only (person, object, or scene)
• Centered composition
• No complex background
• No text

`

	legoStyle ImagePrompt = `
Illustration style:
• LEGO-style illustration
• Everything built from LEGO bricks
• Plastic brick textures and stud details
• Blocky structures and modular construction
• Bright toy-like colors
• Simple toy photography lighting
• Playful miniature scene
• Stylized toy aesthetic

Composition:
• One main subject only (person, object, or scene)
• Centered composition
• No complex background
• No text`

	cartoonStyle ImagePrompt = `
Illustration style:
• Cartoonish illustration
• Exaggerated expressions and body proportions
• Humorous and dynamic poses
• Clean bold outlines
• Bright flat colors
• Playful and slightly chaotic cartoon energy
• Simple editorial cartoon composition

Composition:
• One main subject only (person, object, or scene)
• Centered composition
• No complex background
• No text
`

	editorialCartoonStyle ImagePrompt = `

Illustration style:
• Editorial cartoon illustration
• Satirical political cartoon style
• Hand-drawn ink lines
• Slight exaggeration of characters and expressions
• Newspaper editorial illustration aesthetics
• Simple color fills or minimal shading
• Visual satire and symbolic storytelling

Composition:
• One main subject only (person, object, or scene)
• Centered composition
• No complex background
• No text
`
)

var imgStylePrompts []ImagePrompt = []ImagePrompt{
	colorPencilStyle,
	paperArtStyle,
	legoStyle,
	cartoonStyle,
	editorialCartoonStyle,
}
