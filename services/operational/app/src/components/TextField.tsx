import { StyleSheet, Text, TextInput, View, type TextInputProps } from "react-native";

type Props = TextInputProps & {
  label: string;
};

export function TextField({ label, style, ...inputProps }: Props) {
  return (
    <View style={styles.wrapper}>
      <Text style={styles.label}>{label}</Text>
      <TextInput
        {...inputProps}
        style={[styles.input, style]}
        placeholderTextColor="#9ca3af"
        autoCorrect={false}
        autoCapitalize={inputProps.autoCapitalize ?? "none"}
      />
    </View>
  );
}

const styles = StyleSheet.create({
  wrapper: {
    marginBottom: 12,
  },
  label: {
    fontSize: 13,
    fontWeight: "600",
    color: "#404040",
    marginBottom: 6,
  },
  input: {
    height: 48,
    borderWidth: 1,
    borderColor: "#d4d4d4",
    borderRadius: 8,
    paddingHorizontal: 12,
    fontSize: 16,
    color: "#1a1a1a",
    backgroundColor: "#ffffff",
  },
});
