module Tests exposing (..)

import Expect
import Json.Decode as D
import Main exposing (Namespace, Status, msgDecoder, nsDecoder, statusDecoder)
import Test exposing (..)


boldJson =
    """
        {
            "name": "bold-person",
            "up": true,
            "memUsed": 3,
            "memLimit": 50,
            "startHour": 10,
            "stopTime": 1572735059247
        }
    """


beautifulJson =
    """
            {
                "name": "beautiful-spider",
                "up": false,
                "memUsed": 45,
                "memLimit": 100,
                "stopTime": 1572738659267
            }
    """


creepyJson =
    """
            {
                "name": "creepy-demon",
                "up": false,
                "memUsed": 6,
                "memLimit": 20,
                "startHour": 8
            }
    """


jsonList =
    "[" ++ boldJson ++ "," ++ beautifulJson ++ "," ++ creepyJson ++ "]"


jsonMessage =
    "{\"namespaces\":" ++ jsonList ++ ",\"time\": \"20:14 UTC\"}"


creepy : Namespace
creepy =
    { name = "creepy-demon"
    , up = False
    , memUsed = 6
    , memLimit = 20
    , startHour = Just 8
    , stopTime = Nothing
    , remaining = Nothing
    }


beautiful : Namespace
beautiful =
    { name = "beautiful-spider"
    , up = False
    , memUsed = 45
    , memLimit = 100
    , stopTime = Just 1572738659267
    , startHour = Nothing
    , remaining = Nothing
    }


bold : Namespace
bold =
    { name = "bold-person"
    , up = True
    , memUsed = 3
    , memLimit = 50
    , startHour = Just 10
    , stopTime = Just 1572735059247
    , remaining = Nothing
    }


msg : Test
msg =
    test "Decode full status message" <|
        \_ ->
            Expect.equal
                (msgDecoder jsonMessage)
            <|
                Ok (Status [ bold, beautiful, creepy ] "20:14 UTC")


several : Test
several =
    test "Decode list of namespaces from JSON" <|
        \_ ->
            Expect.equal
                (D.decodeString (D.list nsDecoder) jsonList)
                (Ok [ bold, beautiful, creepy ])


one : Test
one =
    test "Decode namespace from JSON" <|
        \_ ->
            Expect.equal
                (D.decodeString nsDecoder beautifulJson)
                (Ok beautiful)
