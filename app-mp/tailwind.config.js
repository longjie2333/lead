const { iconsPlugin, getIconCollections } = require("@egoist/tailwindcss-icons")

module.exports = {
    theme: {
        extend: {
            colors: {
                'primary': "#3482ff"
            }
        }
    },
    plugins: [
        iconsPlugin({
            // Select the icon collections you want to use
            collections: getIconCollections(["lucide"]),
        }),
    ],
}