# Evidence pack — `Savio09/plantly`

> Auto-generated raw material for the hand-made gold-standard preview (G13).
> It reproduces what the ingestion client sees; it makes **no claims**. Write the preview against this, citing the files below.

_Generated 2026-10-24 via `cmd/evidencepack`._

## Metadata

- **Full name:** Savio09/plantly
- **Description:** A neat React Native app that reminds one to water plants. It's cross platform.
- **Primary language:** TypeScript
- **Stars:** 0
- **License:** —
- **Default branch:** main
- **Last pushed:** 2026-06-18T06:53:53Z
- **Archived:** false · **Fork:** false
- **URL:** https://github.com/Savio09/plantly

## Structure (top 2 levels)

```
.eslintrc.js
.gitignore
README.md
app/
app.json
  (tabs)/
  _layout.tsx
  new.tsx
  onboarding.tsx
assets/
  85620.jpg
  adaptive-icon.png
  detail_info.PNG
  details.PNG
  favicon.png
  form.PNG
  icon.png
  landing.PNG
  onboard.PNG
  plantly.png
  splash.png
babel.config.js
components/
  PlantCard.tsx
  PlantlyButton.tsx
  PlantlyImage.tsx
package-lock.json
package.json
store/
  plantsStore.ts
  userStore.ts
theme.ts
tsconfig.json
```

_Total entries ingested: 40._

## Key files

Selected 5 candidate key files; 5 fetched, 0 skipped.

### `package.json`

```json
{
  "name": "plantly",
  "version": "1.0.0",
  "main": "expo-router/entry",
  "scripts": {
    "start": "expo start",
    "android": "expo start --android",
    "ios": "expo start --ios",
    "web": "expo start --web",
    "lint": "eslint ."
  },
  "dependencies": {
    "@expo/vector-icons": "^14.0.3",
    "@react-native-async-storage/async-storage": "1.23.1",
    "date-fns": "^4.1.0",
    "expo": "~51.0.28",
    "expo-constants": "~16.0.2",
    "expo-file-system": "~17.0.1",
    "expo-haptics": "~13.0.1",
    "expo-image-picker": "~15.0.7",
    "expo-linear-gradient": "~13.0.2",
    "expo-linking": "~6.3.1",
    "expo-router": "~3.5.24",
    "expo-status-bar": "~1.12.1",
    "react": "18.2.0",
    "react-native": "0.74.5",
    "react-native-keyboard-aware-scroll-view": "^0.9.5",
    "react-native-safe-area-context": "4.10.5",
    "react-native-screens": "3.31.1",
    "zustand": "^5.0.1",
    "expo-dev-client": "~4.0.29"
  },
  "devDependencies": {
    "@babel/core": "^7.20.0",
    "@types/react": "~18.2.45",
    "eslint": "^8.57.0",
    "eslint-config-expo": "~7.1.2",
    "eslint-config-prettier": "^9.1.0",
    "eslint-plugin-prettier": "^5.2.1",
    "eslint-plugin-react-native": "^4.1.0",
    "prettier": "^3.3.3",
    "typescript": "^5.1.3"
  },
  "private": true
}
```

### `README.md`

```markdown
# Plantly

This is an application that reminds one to take care of their plants. It's one of the apps that I would be building for my five native apps challenge using React Native. I'll be getting back to this shortly.

## Core Features of the Application

- Allows for Video uploads
- Form to add plant information
- Dynamic Routing with Tab menu
- More features to be added soon

# Technologies Used

- React Native using Typescript
- Zustand for State Management

# Application UI

![Home Screen](assets/landing.PNG)
![Onboarding](assets/onboard.PNG)
![Form](assets/form.PNG)
![List Items](assets/85620.jpg)
![Details](assets/details.PNG)
![More Details](assets/landing.PNG)
```

### `app.json`

```json
{
  "expo": {
    "name": "plantly",
    "slug": "plantly",
    "scheme": "plantly",
    "version": "1.0.0",
    "orientation": "portrait",
    "icon": "./assets/icon.png",
    "userInterfaceStyle": "light",
    "splash": {
      "image": "./assets/splash.png",
      "resizeMode": "contain",
      "backgroundColor": "#ffffff"
    },
    "ios": {
      "supportsTablet": true
    },
    "android": {
      "adaptiveIcon": {
        "foregroundImage": "./assets/adaptive-icon.png",
        "backgroundColor": "#ffffff"
      }
    },
    "web": {
      "favicon": "./assets/favicon.png"
    },
    "plugins": ["expo-router"]
  }
}
```

### `babel.config.js`

```javascript
module.exports = function (api) {
  api.cache(true);
  return {
    presets: ["babel-preset-expo"],
  };
};
```

### `tsconfig.json`

```json
{
  "extends": "expo/tsconfig.base",
  "compilerOptions": {
    "strict": true,
    "paths": {
      "@/*": ["./*"]
    }
  }
}
```

## Source index (cite against these)

- `package.json`
- `README.md`
- `app.json`
- `babel.config.js`
- `tsconfig.json`

