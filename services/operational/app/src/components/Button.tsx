import { ActivityIndicator, Pressable, StyleSheet, Text, type StyleProp, type ViewStyle } from "react-native";

type Props = {
  label: string;
  onPress: () => void;
  disabled?: boolean;
  loading?: boolean;
  variant?: "primary" | "secondary";
  style?: StyleProp<ViewStyle>;
};

export function Button({ label, onPress, disabled, loading, variant = "primary", style }: Props) {
  const inert = disabled || loading;
  const base = variant === "primary" ? styles.primary : styles.secondary;
  return (
    <Pressable
      onPress={onPress}
      disabled={inert}
      style={({ pressed }) => [styles.button, base, inert && styles.disabled, pressed && !inert && styles.pressed, style]}
    >
      {loading ? (
        <ActivityIndicator color={variant === "primary" ? "#fff" : "#1a1a1a"} />
      ) : (
        <Text style={[styles.label, variant === "primary" ? styles.labelPrimary : styles.labelSecondary]}>
          {label}
        </Text>
      )}
    </Pressable>
  );
}

const styles = StyleSheet.create({
  button: {
    height: 48,
    borderRadius: 8,
    alignItems: "center",
    justifyContent: "center",
    paddingHorizontal: 16,
  },
  primary: {
    backgroundColor: "#1a1a1a",
  },
  secondary: {
    backgroundColor: "#ffffff",
    borderWidth: 1,
    borderColor: "#d4d4d4",
  },
  disabled: {
    opacity: 0.5,
  },
  pressed: {
    opacity: 0.8,
  },
  label: {
    fontSize: 16,
    fontWeight: "600",
  },
  labelPrimary: {
    color: "#ffffff",
  },
  labelSecondary: {
    color: "#1a1a1a",
  },
});
