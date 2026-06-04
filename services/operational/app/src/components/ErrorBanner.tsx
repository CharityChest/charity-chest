import { StyleSheet, Text, View } from "react-native";

type Props = {
  message: string | null;
};

// Inline error banner. Renders nothing when message is null so callers can
// pass state straight in without conditional wrapping.
export function ErrorBanner({ message }: Props) {
  if (!message) return null;
  return (
    <View style={styles.wrapper}>
      <Text style={styles.text}>{message}</Text>
    </View>
  );
}

const styles = StyleSheet.create({
  wrapper: {
    backgroundColor: "#fee2e2",
    borderColor: "#fca5a5",
    borderWidth: 1,
    borderRadius: 8,
    padding: 12,
    marginBottom: 12,
  },
  text: {
    color: "#991b1b",
    fontSize: 14,
  },
});
